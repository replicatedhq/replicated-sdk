{{/*
Expand the name of the chart.
*/}}
{{- define "replicated-sdk.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "replicated-sdk.fullname" -}}
  {{- if .Values.fullnameOverride }}
    {{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
  {{- else }}
    {{- $name := default .Chart.Name .Values.nameOverride }}
    {{- if contains $name .Release.Name }}
      {{- .Release.Name | trunc 63 | trimSuffix "-" }}
    {{- else }}
      {{- printf "%s" $name | trunc 63 | trimSuffix "-" }}
    {{- end }}
  {{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "replicated-sdk.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Allow the release namespace to be overridden for multi-namespace deployments.
*/}}
{{- define "replicated-sdk.namespace" -}}
{{- if .Values.namespaceOverride -}}
{{- .Values.namespaceOverride -}}
{{- else -}}
{{- .Release.Namespace -}}
{{- end -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "replicated-sdk.labels" -}}
helm.sh/chart: {{ include "replicated-sdk.chart" . }}
{{ include "replicated-sdk.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "replicated-sdk.selectorLabels" -}}
app.kubernetes.io/name: {{ include "replicated-sdk.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
License Fields
*/}}
{{- define "replicated-sdk.licenseFields" -}}
  {{- if (((.Values.global).replicated).licenseFields) -}}
    {{- .Values.global.replicated.licenseFields | toYaml -}}
  {{- else if .Values.licenseFields -}}
    {{- .Values.licenseFields | toYaml -}}
  {{- end -}}
{{- end -}}

{{/*
Is OpenShift
*/}}
{{- define "replicated-sdk.isOpenShift" -}}
  {{- $isOpenShift := false }}
  {{- range .Capabilities.APIVersions -}}
    {{- if hasPrefix "apps.openshift.io/" . -}}
      {{- $isOpenShift = true }}
    {{- end -}}
  {{- end -}}
  {{- $isOpenShift }}
{{- end }}
