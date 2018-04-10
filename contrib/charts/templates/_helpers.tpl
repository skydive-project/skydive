{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Helper functions which can be used for used for .Values.arch in PPA Charts
Check if tag contains specific platform suffix and if not set based on kube platform
uncomment this section for PPA charts, can be removed in github.com charts

{{- define "platform" -}}
{{- if not .Values.arch }}
  {{- if (eq "linux/amd64" .Capabilities.KubeVersion.Platform) }}
    {{- printf "-%s" "x86_64" }}
  {{- end -}}
  {{- if (eq "linux/ppc64le" .Capabilities.KubeVersion.Platform) }}
    {{- printf "-%s" "ppc64le" }}
  {{- end -}}
{{- else -}}
  {{- if eq .Values.arch "amd64" }}
    {{- printf "-%s" "x86_64" }}
  {{- else -}}
    {{- printf "-%s" .Values.arch }}
  {{- end -}}
{{- end -}}
{{- end -}}

{{- define "arch" -}}
  {{- if (eq "linux/amd64" .Capabilities.KubeVersion.Platform) }}
    {{- printf "%s" "amd64" }}
  {{- end -}}
  {{- if (eq "linux/ppc64le" .Capabilities.KubeVersion.Platform) }}
    {{- printf "%s" "ppc64le" }}
  {{- end -}}
{{- end -}}
		
*/}}
	