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
Release Images
Looks up list of images from values provided by the parent app/release. Supports both
global.replicated.releaseImages and replicated.releaseImages locations.
*/}}
{{- define "replicated.releaseImages" -}}
  {{- if and .Values.global .Values.global.replicated .Values.global.replicated.releaseImages -}}
    {{- .Values.global.replicated.releaseImages | toYaml -}}
  {{- else if and .Values.replicated .Values.replicated.releaseImages -}}
    {{- .Values.replicated.releaseImages | toYaml -}}
  {{- else if .Values.releaseImages -}}
    {{- .Values.releaseImages | toYaml -}}
  {{- end -}}
{{- end -}}

{{/*
Detect if we're running on OpenShift
*/}}
{{- define "replicated.isOpenShift" -}}
{{- $isOpenShift := false }}
{{- range .Capabilities.APIVersions -}}
  {{- if hasPrefix "apps.openshift.io/" . -}}
    {{- $isOpenShift = true }}
  {{- end -}}
{{- end -}}
{{- $isOpenShift }}
{{- end -}}

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

{{/*
Process pod security context for OpenShift compatibility
*/}}
{{- define "replicated.podSecurityContext" -}}
{{- $isOpenShift := eq (include "replicated.isOpenShift" .) "true" }}
{{- $podSecurityContext := .Values.podSecurityContext | deepCopy }}
{{- if $podSecurityContext }}
{{- if $isOpenShift }}
  {{- if eq ($podSecurityContext.runAsUser | int) 1001 }}
    {{- $_ := unset $podSecurityContext "runAsUser" }}
  {{- end }}
  {{- if eq ($podSecurityContext.runAsGroup | int) 1001 }}
    {{- $_ := unset $podSecurityContext "runAsGroup" }}
  {{- end }}
  {{- if eq ($podSecurityContext.fsGroup | int) 1001 }}
    {{- $_ := unset $podSecurityContext "fsGroup" }}
  {{- end }}
  {{- if $podSecurityContext.supplementalGroups }}
    {{- $hasOnly1001 := true }}
    {{- range $podSecurityContext.supplementalGroups }}
      {{- if ne (. | int) 1001 }}
        {{- $hasOnly1001 = false }}
      {{- end }}
    {{- end }}
    {{- if $hasOnly1001 }}
      {{- $_ := unset $podSecurityContext "supplementalGroups" }}
    {{- end }}
  {{- end }}
{{- end }}
{{- if hasKey $podSecurityContext "enabled" }}
  {{- $_ := unset $podSecurityContext "enabled" }}
{{- end }}
{{- end }}
{{- toYaml $podSecurityContext }}
{{- end -}}

{{/*
Process container security context
*/}}
{{- define "replicated.containerSecurityContext" -}}
{{- $containerSecurityContext := .Values.containerSecurityContext | deepCopy }}
{{- if $containerSecurityContext }}
{{- if hasKey $containerSecurityContext "enabled" }}
  {{- $_ := unset $containerSecurityContext "enabled" }}
{{- end }}
{{- end }}
{{- toYaml $containerSecurityContext }}
{{- end -}}

{{/*
Get the secret name to use - either the user-specified existing secret or the default
*/}}
{{- define "replicated.secretName" -}}
{{- if and .Values.existingSecret .Values.existingSecret.name -}}
  {{- .Values.existingSecret.name -}}
{{- else -}}
  {{ include "replicated.name" . }}
{{- end -}}
{{- end -}}

{{/*
Parse status informers from the license fields
*/}}

{{/*
Get all status informers as a list of resource/name pairs
*/}}
{{- define "replicated.statusInformers" -}}
{{- .Values.statusInformers | default list -}}
{{- end -}}

{{/*
Get deployment names from status informers
*/}}
{{- define "replicated.statusInformers.deployments" -}}
{{- $deployments := list -}}
{{- if .Values.statusInformers -}}
{{- range .Values.statusInformers -}}
  {{- if hasPrefix "deployment/" . -}}
    {{- $name := trimPrefix "deployment/" . -}}
    {{- $deployments = append $deployments $name -}}
  {{- end -}}
{{- end -}}
{{- end -}}
{{- range $deployments }}
- {{ . }}
{{- end -}}
{{- end -}}

{{/*
Get statefulset names from status informers
*/}}
{{- define "replicated.statusInformers.statefulsets" -}}
{{- $statefulsets := list -}}
{{- if .Values.statusInformers -}}
{{- range .Values.statusInformers -}}
  {{- if hasPrefix "statefulset/" . -}}
    {{- $name := trimPrefix "statefulset/" . -}}
    {{- $statefulsets = append $statefulsets $name -}}
  {{- end -}}
{{- end -}}
{{- end -}}
{{- range $statefulsets }}
- {{ . }}
{{- end -}}
{{- end -}}

{{/*
Get service names from status informers
*/}}
{{- define "replicated.statusInformers.services" -}}
{{- $services := list -}}
{{- if .Values.statusInformers -}}
{{- range .Values.statusInformers -}}
  {{- if hasPrefix "service/" . -}}
    {{- $name := trimPrefix "service/" . -}}
    {{- $services = append $services $name -}}
  {{- end -}}
{{- end -}}
{{- end -}}
{{- range $services }}
- {{ . }}
{{- end -}}
{{- end -}}

{{/*
Get ingress names from status informers
*/}}
{{- define "replicated.statusInformers.ingresses" -}}
{{- $ingresses := list -}}
{{- if .Values.statusInformers -}}
{{- range .Values.statusInformers -}}
  {{- if hasPrefix "ingress/" . -}}
    {{- $name := trimPrefix "ingress/" . -}}
    {{- $ingresses = append $ingresses $name -}}
  {{- end -}}
{{- end -}}
{{- end -}}
{{- range $ingresses }}
- {{ . }}
{{- end -}}
{{- end -}}

{{/*
Get PVC names from status informers
*/}}
{{- define "replicated.statusInformers.pvcs" -}}
{{- $pvcs := list -}}
{{- if .Values.statusInformers -}}
{{- range .Values.statusInformers -}}
  {{- if hasPrefix "pvc/" . -}}
    {{- $name := trimPrefix "pvc/" . -}}
    {{- $pvcs = append $pvcs $name -}}
  {{- end -}}
{{- end -}}
{{- end -}}
{{- range $pvcs }}
- {{ . }}
{{- end -}}
{{- end -}}

{{/*
Get daemonset names from status informers
*/}}
{{- define "replicated.statusInformers.daemonsets" -}}
{{- $daemonsets := list -}}
{{- if .Values.statusInformers -}}
{{- range .Values.statusInformers -}}
  {{- if hasPrefix "daemonset/" . -}}
    {{- $name := trimPrefix "daemonset/" . -}}
    {{- $daemonsets = append $daemonsets $name -}}
  {{- end -}}
{{- end -}}
{{- end -}}
{{- range $daemonsets }}
- {{ . }}
{{- end -}}
{{- end -}}