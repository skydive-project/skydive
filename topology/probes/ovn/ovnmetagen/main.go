package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/ovn-kubernetes/libovsdb/modelgen"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of ovnmetagen:\n")
	fmt.Fprintf(os.Stderr, "\tovnmetagen [flags] OVS_SCHEMA\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}

var (
	outDirP  = flag.String("o", ".", "Directory where the generated files shall be stored")
	pkgNameP = flag.String("p", "ovsmodel", "Package name")
	dryRun   = flag.Bool("d", false, "Dry run")
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("ovnmetagen: ")
	flag.Usage = usage
	flag.Parse()
	outDir := *outDirP
	pkgName := *pkgNameP

	outDir, err := filepath.Abs(outDir)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatal(err)
	}

	if len(flag.Args()) != 1 {
		flag.Usage()
		os.Exit(2)
	}

	schemaFile, err := os.Open(flag.Args()[0])
	if err != nil {
		log.Fatal(err)
	}
	defer schemaFile.Close()

	schemaBytes, err := io.ReadAll(schemaFile)
	if err != nil {
		log.Fatal(err)
	}

	var dbSchema ovsdb.DatabaseSchema
	if err := json.Unmarshal(schemaBytes, &dbSchema); err != nil {
		log.Fatal(err)
	}

	genOpts := []modelgen.Option{}
	if *dryRun {
		genOpts = append(genOpts, modelgen.WithDryRun())
	}
	generator, err := modelgen.NewGenerator(genOpts...)
	if err != nil {
		log.Fatal(err)
	}

	for name := range dbSchema.Tables {
		templ, data := tableTemplate(pkgName, name, &dbSchema)
		err := generator.Generate(modelgen.FileName(name), templ, data)
		if err != nil {
			log.Printf("Error creating table template: %s", name)
			log.Fatal(err)
		}
	}

	dbTempl, dbData := modelTemplate(pkgName, dbSchema)
	if err != nil {
		log.Print("Error creating DBModel template")
		log.Fatal(err)
	}
	err = generator.Generate("model.go", dbTempl, dbData)
	if err != nil {
		log.Fatal(err)
	}

	linksTempl, lData := linksTemplate(pkgName, &dbSchema)
	if err != nil {
		log.Print("Error creating links template")
		log.Fatal(err)
	}
	err = generator.Generate("links.go", linksTempl, lData)
	if err != nil {
		log.Fatal(err)
	}
}
