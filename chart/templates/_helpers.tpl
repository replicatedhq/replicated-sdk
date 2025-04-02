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
{{- with .Values.commonLabels }}
{{- toYaml . | nindent 0 }}
{{- end }}
{{- end }}

{{/* 
Pod Labels
*/}}
{{- define "replicated.podLabels" -}}
{{- with .Values.podLabels }}
{{- toYaml . | nindent 0 }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "replicated.selectorLabels" -}}
app.kubernetes.io/name: {{ include "replicated.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Image
*/}}
{{- define "replicated.image" -}}
{{- $registryName := default .Values.image.registry ((.Values.global).imageRegistry) -}}
{{- $repositoryName := .Values.image.repository -}}
{{- $separator := ":" -}}
{{- $termination := "" -}}

{{- if not $repositoryName -}}
  {{- fail "Image repository is required but not set" -}}
{{- end -}}

{{- if .Values.image.tag -}}
  {{- $termination = .Values.image.tag | toString -}}
{{- else if .Chart -}}
  {{- $termination = .Chart.AppVersion | default "latest" | toString -}}
{{- else -}}
  {{- $termination = "latest" -}}
{{- end -}}

{{- if $registryName -}}
  {{- printf "%s/%s%s%s" $registryName $repositoryName $separator $termination -}}
{{- else -}}
  {{- printf "%s%s%s" $repositoryName $separator $termination -}}
{{- end -}}
{{- end -}}

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

{{/*
Get the Replicated App Endpoint
*/}}
{{- define "replicated.appEndpoint" -}}
{{- if .Values.replicatedAppDomain -}}
  {{- printf "https://%s" .Values.replicatedAppDomain -}}
{{- else if .Values.replicatedAppEndpoint -}}
  {{- .Values.replicatedAppEndpoint -}}
{{- else -}}
  {{- printf "https://replicated.app" -}}
{{- end -}}
{{- end -}}

{{/*
Adjust the pod security context for OpenShift compatibility
OpenShift runs pods with a specific UID (1001) and handles security contexts differently
This helper removes those fields if they match OpenShift defaults to avoid conflicts
*/}}
{{- define "replicated.openShiftSecurityContext" -}}
{{- $podSecurityContext := deepCopy . -}}
{{- if eq (include "replicated.isOpenShift" .) "true" -}}
  {{- if eq ($podSecurityContext.runAsUser | int) 1001 -}}
    {{- $_ := unset $podSecurityContext "runAsUser" -}}
  {{- end -}}
  {{- if eq ($podSecurityContext.runAsGroup | int) 1001 -}}
    {{- $_ := unset $podSecurityContext "runAsGroup" -}}
  {{- end -}}
  {{- if eq ($podSecurityContext.fsGroup | int) 1001 -}}
    {{- $_ := unset $podSecurityContext "fsGroup" -}}
  {{- end -}}
  {{- if eq ($podSecurityContext.supplementalGroups | len) 1 -}}
    {{- if eq (index $podSecurityContext.supplementalGroups 0 | int) 1001 -}}
      {{- $_ := unset $podSecurityContext "supplementalGroups" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- $podSecurityContext -}}
{{- end -}}

{{/*
Return the proper container image
This helper handles both the legacy .Values.images format and the new
structured .Values.image format, selecting the appropriate one based on what's defined.
*/}}
{{- define "replicated.containerImage" -}}
{{- if .Values.images -}}
    {{- index .Values.images "replicated-sdk" -}}
{{- else -}}
    {{- include "replicated.image" . -}}
{{- end -}}
{{- end -}}

{{/*
Return the proper image pull policy
*/}}
{{- define "replicated.imagePullPolicy" -}}
{{- if .Values.images -}}
    {{- print "IfNotPresent" -}}
{{- else -}}
    {{- .Values.image.pullPolicy | default "IfNotPresent" -}}
{{- end -}}
{{- end -}}
