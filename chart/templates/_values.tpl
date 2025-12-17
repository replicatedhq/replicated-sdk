{{/*
Helm Lookup Helpers for Replicated SDK

Look up values from an existing secret and merge onto .Values.
The secret should contain a `values.yaml` key with YAML in the same structure as values.yaml.

Lookup returns empty during `helm template` (no cluster) - falls back to values.yaml.
*/}}

{{/*
replicated.lookupValues - Parse values from existing secret
*/}}
{{- define "replicated.lookupValues" -}}
{{- $result := dict -}}
{{- if and .Values.valuesFrom .Values.valuesFrom.secretName -}}
  {{- $secret := lookup "v1" "Secret" (include "replicated.namespace" .) .Values.valuesFrom.secretName -}}
  {{- if and $secret $secret.data (index $secret.data "values.yaml") -}}
    {{- $result = index $secret.data "values.yaml" | b64dec | fromYaml -}}
  {{- end -}}
{{- end -}}
{{- $result | toYaml -}}
{{- end -}}

{{/*
replicated.values - Merge lookup values onto .Values
*/}}
{{- define "replicated.values" -}}
{{- $lookupValues := include "replicated.lookupValues" . | fromYaml -}}
{{- merge $lookupValues .Values | toYaml -}}
{{- end -}}

{{/*
replicated.hasLookupValues - Check if lookup returned data
*/}}
{{- define "replicated.hasLookupValues" -}}
{{- $lookupValues := include "replicated.lookupValues" . | fromYaml -}}
{{- if $lookupValues }}true{{- else }}false{{- end -}}
{{- end -}}
