{{/*
Expand the name of the chart.
*/}}
{{- define "replicated.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "replicated.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Allow the release namespace to be overridden for multi-namespace deployments.
*/}}
{{- define "replicated.namespace" -}}
{{- if .Values.namespaceOverride -}}
{{- .Values.namespaceOverride -}}
{{- else -}}
{{- .Release.Namespace -}}
{{- end -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "replicated.labels" -}}
helm.sh/chart: {{ include "replicated.chart" . }}
{{ include "replicated.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "replicated.selectorLabels" -}}
app.kubernetes.io/name: {{ include "replicated.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
License Fields
*/}}
{{- define "replicated.licenseFields" -}}
  {{- if (((.Values.global).replicated).licenseFields) -}}
    {{- .Values.global.replicated.licenseFields | toYaml -}}
  {{- else if .Values.licenseFields -}}
    {{- .Values.licenseFields | toYaml -}}
  {{- end -}}
{{- end -}}

{{/*
Is OpenShift
*/}}
{{- define "replicated.isOpenShift" -}}
  {{- $isOpenShift := false }}
  {{- range .Capabilities.APIVersions -}}
    {{- if hasPrefix "apps.openshift.io/" . -}}
      {{- $isOpenShift = true }}
    {{- end -}}
  {{- end -}}
  {{- $isOpenShift }}
{{- end }}

{{/*
Resource Names
*/}}
{{- define "replicated.deploymentName" -}}
  {{ include "replicated.name" . }}
{{- end -}}

{{- define "replicated.roleName" -}}
  {{ include "replicated.name" . }}-role
{{- end -}}

{{- define "replicated.roleBindingName" -}}
  {{ include "replicated.name" . }}-rolebinding
{{- end -}}

{{- define "replicated.secretName" -}}
  {{ include "replicated.name" . }}
{{- end -}}

{{- define "replicated.serviceName" -}}
  {{ include "replicated.name" . }}
{{- end -}}

{{- define "replicated.serviceAccountName" -}}
  {{ .Values.serviceAccountName | default (include "replicated.name" .) }}
{{- end -}}

{{- define "replicated.supportBundleName" -}}
  {{ include "replicated.name" . }}-supportbundle
{{- end -}}
