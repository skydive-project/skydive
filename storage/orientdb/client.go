/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package orientdb

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/storage"
)

// Result describes an orientdb request result
type Result struct {
	Body []byte
}

// ClientInterface describes the mechanism API of OrientDB database client
type ClientInterface interface {
	Request(method string, url string, body io.Reader) (*http.Response, error)
	DeleteDocument(id string) error
	GetDocument(id string) (*Result, error)
	CreateDocument(doc interface{}) (*Result, error)
	Upsert(class string, doc interface{}, idkey string, idval string) (*Result, error)
	GetDocumentClass(name string) (*DocumentClass, error)
	AlterProperty(className string, prop Property) error
	CreateProperty(className string, prop Property) error
	CreateClass(class ClassDefinition) error
	CreateIndex(className string, index Index) error
	CreateDocumentClass(class ClassDefinition) error
	DeleteDocumentClass(name string) error
	GetDatabase() (*Result, error)
	CreateDatabase() (*Result, error)
	SQL(query string) (*Result, error)
	Query(obj string, query *filters.SearchQuery) (*Result, error)
	Connect() error
	AddEventListener(listener storage.EventListener)
}

// Client describes a OrientDB client database
type Client struct {
	sync.RWMutex
	url           string
	authenticated bool
	database      string
	username      string
	password      string
	cookies       []*http.Cookie
	client        *http.Client
	listeners     []storage.EventListener
}

// Session describes a OrientDB client session
type Session struct {
	client   *Client
	database string
}

// Error describes a OrientDB error
type Error struct {
	Code    int    `json:"code"`
	Reason  int    `json:"reason"`
	Content string `json:"content"`
}

// Errors describes a list of OrientDB errors
type Errors struct {
	Errors []Error `json:"errors"`
}

// Property describes a OrientDB property
type Property struct {
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	LinkedType  string `json:"linkedType,omitempty"`
	LinkedClass string `json:"linkedClass,omitempty"`
	Mandatory   bool   `json:"mandatory"`
	NotNull     bool   `json:"notNull"`
	ReadOnly    bool   `json:"readonly"`
	Collate     string `json:"collate,omitempty"`
	Regexp      string `json:"regexp,omitempty"`
}

// Index describes a OrientDB index
type Index struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Fields []string `json:"fields"`
}

// ClassDefinition describes a OrientDB class definition
type ClassDefinition struct {
	Name         string     `json:"name"`
	SuperClass   string     `json:"superClass,omitempty"`
	SuperClasses []string   `json:"superClasses,omitempty"`
	Abstract     bool       `json:"abstract"`
	StrictMode   bool       `json:"strictmode"`
	Alias        string     `json:"alias,omitempty"`
	Properties   []Property `json:"properties,omitempty"`
	Indexes      []Index    `json:"indexes,omitempty"`
}

// DocumentClass describes OrientDB document
type DocumentClass struct {
	Class ClassDefinition `json:"class"`
}

func parseError(body io.Reader) error {
	var errs Errors
	if err := common.JSONDecode(body, &errs); err != nil {
		return fmt.Errorf("Error while parsing error: %s (%s)", err, body)
	}
	var s string
	for _, err := range errs.Errors {
		s += err.Content + "\n"
	}
	return errors.New(s)
}

func getResponseBody(resp *http.Response) (io.ReadCloser, error) {
	if encoding := resp.Header.Get("Content-Encoding"); encoding == "gzip" {
		decompressor, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		return decompressor, nil
	}
	return resp.Body, nil
}

func parseResponse(resp *http.Response) (*Result, error) {
	if resp.StatusCode < 400 && resp.ContentLength == 0 {
		return nil, nil
	}

	body, err := getResponseBody(resp)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	if resp.StatusCode >= 400 {
		return nil, parseError(body)
	}

	var result Result
	result.Body, err = ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func compressBody(body io.Reader) io.Reader {
	buffer := new(bytes.Buffer)
	compressor := gzip.NewWriter(buffer)
	io.Copy(compressor, body)
	compressor.Close()
	return buffer
}

// FilterToExpression returns a OrientDB select expression based on filters
func FilterToExpression(f *filters.Filter, formatter func(string) string) string {
	if formatter == nil {
		formatter = func(s string) string { return s }
	}

	if f.BoolFilter != nil {
		keyword := ""
		switch f.BoolFilter.Op {
		case filters.BoolFilterOp_NOT:
			return "NOT (" + FilterToExpression(f.BoolFilter.Filters[0], formatter) + ")"
		case filters.BoolFilterOp_OR:
			keyword = "OR"
		case filters.BoolFilterOp_AND:
			keyword = "AND"
		}
		var conditions []string
		for _, item := range f.BoolFilter.Filters {
			if expr := FilterToExpression(item, formatter); expr != "" {
				conditions = append(conditions, "("+FilterToExpression(item, formatter)+")")
			}
		}
		return strings.Join(conditions, " "+keyword+" ")
	}

	if f.TermStringFilter != nil {
		return fmt.Sprintf(`(%s = "%s") OR ("%s" IN %s)`, formatter(f.TermStringFilter.Key), f.TermStringFilter.Value,
			f.TermStringFilter.Value, formatter(f.TermStringFilter.Key))
	}

	if f.TermInt64Filter != nil {
		return fmt.Sprintf(`(%s = %d) OR (%d IN %s)`, formatter(f.TermInt64Filter.Key), f.TermInt64Filter.Value,
			f.TermInt64Filter.Value, formatter(f.TermInt64Filter.Key))
	}

	if f.TermBoolFilter != nil {
		return fmt.Sprintf(`(%s = %s) OR (%s IN %s)`, formatter(f.TermBoolFilter.Key), strconv.FormatBool(f.TermBoolFilter.Value),
			strconv.FormatBool(f.TermBoolFilter.Value), formatter(f.TermBoolFilter.Key))
	}

	if f.GtInt64Filter != nil {
		return fmt.Sprintf("%v > %v", formatter(f.GtInt64Filter.Key), f.GtInt64Filter.Value)
	}

	if f.LtInt64Filter != nil {
		return fmt.Sprintf("%v < %v", formatter(f.LtInt64Filter.Key), f.LtInt64Filter.Value)
	}

	if f.GteInt64Filter != nil {
		return fmt.Sprintf("%v >= %v", formatter(f.GteInt64Filter.Key), f.GteInt64Filter.Value)
	}

	if f.LteInt64Filter != nil {
		return fmt.Sprintf("%v <= %v", formatter(f.LteInt64Filter.Key), f.LteInt64Filter.Value)
	}

	if f.RegexFilter != nil {
		return fmt.Sprintf(`%s MATCHES "%s"`, formatter(f.RegexFilter.Key), strings.Replace(f.RegexFilter.Value, `\`, `\\`, -1))
	}

	if f.NullFilter != nil {
		return fmt.Sprintf("%s is NULL", formatter(f.NullFilter.Key))
	}

	if f.IPV4RangeFilter != nil {
		// ignore the error at this point it should have been catched earlier
		regex, _ := common.IPV4CIDRToRegex(f.IPV4RangeFilter.Value)

		return fmt.Sprintf(`%s MATCHES "%s"`, formatter(f.IPV4RangeFilter.Key), strings.Replace(regex, `\`, `\\`, -1))
	}

	return ""
}

// NewClient creates a new OrientDB database client
func NewClient(url string, database string, username string, password string) (*Client, error) {
	client := &Client{
		url:      url,
		database: database,
		username: username,
		password: password,
		client:   &http.Client{},
	}

	_, err := client.GetDatabase()
	if err != nil {
		if _, err := client.CreateDatabase(); err != nil {
			return nil, err
		}
	}

	return client, nil
}

// Connect to the OrientDB server
func (c *Client) Connect() error {
	if err := c.reconnect(); err != nil {
		return err
	}

	c.RLock()
	for _, l := range c.listeners {
		l.OnStarted()
	}
	c.RUnlock()

	return nil
}

// AddEventListener add event listener
func (c *Client) AddEventListener(listener storage.EventListener) {
	c.Lock()
	c.listeners = append(c.listeners, listener)
	c.Unlock()
}

// Request send a request to the OrientDB server
func (c *Client) Request(method string, url string, body io.Reader) (*http.Response, error) {
	if body != nil {
		body = compressBody(body)
	}

	request, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if !c.authenticated {
		request.SetBasicAuth(c.username, c.password)
	} else {
		for _, cookie := range c.cookies {
			request.AddCookie(cookie)
		}
	}

	resp, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 401 {
		if err := c.reconnect(); err != nil {
			return nil, err
		}
		resp, err = c.client.Do(request)
	}

	return resp, err
}

// DeleteDocument delete an OrientDB document
func (c *Client) DeleteDocument(id string) error {
	url := fmt.Sprintf("%s/document/%s/%s", c.url, c.database, id)
	resp, err := c.Request("DELETE", url, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return parseError(resp.Body)
	}
	return nil
}

// GetDocument retrieve a specific OrientDB document
func (c *Client) GetDocument(id string) (*Result, error) {
	url := fmt.Sprintf("%s/document/%s/%s", c.url, c.database, id)
	resp, err := c.Request("GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return parseResponse(resp)
}

// CreateDocument creates an OrientDB document
func (c *Client) CreateDocument(doc interface{}) (*Result, error) {
	url := fmt.Sprintf("%s/document/%s", c.url, c.database)
	marshal, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	resp, err := c.Request("POST", url, bytes.NewBuffer(marshal))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return parseResponse(resp)
}

// Upsert updates or insert a key in an OrientDB document
func (c *Client) Upsert(class string, doc interface{}, idkey string, idval string) (*Result, error) {
	content, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("UPDATE %s CONTENT %s UPSERT RETURN AFTER @rid WHERE %s = '%s'", class, string(content), idkey, idval)
	return c.SQL(query)
}

// GetDocumentClass returns an OrientDB document class
func (c *Client) GetDocumentClass(name string) (*DocumentClass, error) {
	url := fmt.Sprintf("%s/class/%s/%s", c.url, c.database, name)
	resp, err := c.Request("GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result, err := parseResponse(resp)
	if err != nil {
		return nil, err
	}

	var dc DocumentClass
	if err := json.Unmarshal(result.Body, &dc); err != nil {
		return nil, err
	}

	return &dc, nil
}

// AlterProperty modify a property
func (c *Client) AlterProperty(className string, prop Property) error {
	alterQuery := fmt.Sprintf("ALTER PROPERTY %s.%s", className, prop.Name)
	if prop.Mandatory {
		if _, err := c.SQL(alterQuery + " MANDATORY true"); err != nil && err != io.EOF {
			return err
		}
	}
	if prop.NotNull {
		if _, err := c.SQL(alterQuery + " NOTNULL true"); err != nil && err != io.EOF {
			return err
		}
	}
	if prop.ReadOnly {
		if _, err := c.SQL(alterQuery + " READONLY true"); err != nil && err != io.EOF {
			return err
		}
	}
	return nil
}

// CreateProperty creates a new class property
func (c *Client) CreateProperty(className string, prop Property) error {
	query := fmt.Sprintf("CREATE PROPERTY %s.%s %s", className, prop.Name, prop.Type)
	if prop.LinkedClass != "" {
		query += " " + prop.LinkedClass
	}
	if prop.LinkedType != "" {
		query += " " + prop.LinkedType
	}
	if _, err := c.SQL(query); err != nil {
		return err
	}

	return c.AlterProperty(className, prop)
}

// CreateClass creates a new class
func (c *Client) CreateClass(class ClassDefinition) error {
	query := fmt.Sprintf("CREATE CLASS %s", class.Name)
	if class.SuperClass != "" {
		query += " EXTENDS " + class.SuperClass
	}

	if _, err := c.SQL(query); err != nil {
		return err
	}

	return nil
}

// CreateIndex creates a new Index
func (c *Client) CreateIndex(className string, index Index) error {
	query := fmt.Sprintf("CREATE INDEX %s ON %s (%s) %s", index.Name, className, strings.Join(index.Fields, ", "), index.Type)
	if _, err := c.SQL(query); err != nil {
		return err
	}

	return nil
}

// CreateDocumentClass creates a new OrientDB document class
func (c *Client) CreateDocumentClass(class ClassDefinition) error {
	if err := c.CreateClass(class); err != nil {
		return err
	}

	for _, prop := range class.Properties {
		if err := c.CreateProperty(class.Name, prop); err != nil {
			return err
		}
	}

	for _, index := range class.Indexes {
		if err := c.CreateIndex(class.Name, index); err != nil {
			return err
		}
	}

	return nil
}

// DeleteDocumentClass delete an OrientDB document class
func (c *Client) DeleteDocumentClass(name string) error {
	url := fmt.Sprintf("%s/class/%s/%s", c.url, c.database, name)
	resp, err := c.Request("DELETE", url, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return parseError(resp.Body)
	}
	return nil
}

// GetDatabase returns the root OrientDB document
func (c *Client) GetDatabase() (*Result, error) {
	url := fmt.Sprintf("%s/database/%s", c.url, c.database)
	resp, err := c.Request("GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return parseResponse(resp)
}

// CreateDatabase creates the root OrientDB Document
func (c *Client) CreateDatabase() (*Result, error) {
	url := fmt.Sprintf("%s/database/%s/plocal", c.url, c.database)
	resp, err := c.Request("POST", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// OrientDB returns a 500 error but successfully creates the DB
	result, err := parseResponse(resp)
	if err != nil {
		return nil, err
	}

	if _, e := c.GetDatabase(); e != nil {
		// Returns the original error
		return nil, err
	}

	return result, nil
}

// SQL Simple Query Language, send a query to the OrientDB server
func (c *Client) SQL(query string) (*Result, error) {
	url := fmt.Sprintf("%s/command/%s/sql", c.url, c.database)
	resp, err := c.Request("POST", url, bytes.NewBufferString(query))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return parseResponse(resp)
}

// Query the OrientDB based on filters
func (c *Client) Query(obj string, query *filters.SearchQuery) (*Result, error) {
	interval := query.PaginationRange
	filter := query.Filter

	sql := "SELECT FROM " + obj
	if conditional := FilterToExpression(filter, nil); conditional != "" {
		sql += " WHERE " + conditional
	}

	if interval != nil {
		sql += fmt.Sprintf(" LIMIT %d, %d", interval.To-interval.From, interval.From)
	}

	if query.Sort {
		sql += " ORDER BY " + query.SortBy

		if query.SortOrder != "" {
			sql += " " + strings.ToUpper(query.SortOrder)
		}
	}

	return c.SQL(sql)
}

func (c *Client) reconnect() error {
	url := fmt.Sprintf("%s/connect/%s", c.url, c.database)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	request.SetBasicAuth(c.username, c.password)
	resp, err := c.client.Do(request)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Failed to authenticate to OrientDB: %s", resp.Status)
	}

	if resp.StatusCode < 400 && len(resp.Cookies()) != 0 {
		c.authenticated = true
		c.cookies = resp.Cookies()
	}

	return nil
}
