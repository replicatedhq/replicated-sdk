{{/*
Expand the name of the chart.
*/}}
{{- define "kots-sdk.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "kots-sdk.fullname" -}}
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
{{- define "kots-sdk.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kots-sdk.labels" -}}
helm.sh/chart: {{ include "kots-sdk.chart" . }}
{{ include "kots-sdk.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kots-sdk.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kots-sdk.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
License Fields
*/}}
{{- define "licenseFields" -}}
{{- if .Values.global -}}
{{- if .Values.global.licenseFields -}}
{{- .Values.global.licenseFields | toYaml -}}
{{- end -}}
{{- else if .Values.licenseFields -}}
{{- .Values.licenseFields | toYaml -}}
{{- else -}}
"{}"
{{- end -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "kots-sdk.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "kots-sdk.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}