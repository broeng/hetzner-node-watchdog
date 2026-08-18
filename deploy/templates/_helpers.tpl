{{- define "hetzner-node-watchdog.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "hetzner-node-watchdog.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "hetzner-node-watchdog.labels" -}}
app.kubernetes.io/name: {{ include "hetzner-node-watchdog.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "hetzner-node-watchdog.selectorLabels" -}}
app.kubernetes.io/name: {{ include "hetzner-node-watchdog.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "hetzner-node-watchdog.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "hetzner-node-watchdog.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "hetzner-node-watchdog.secretName" -}}
{{- .Values.hcloud.existingSecret | default (include "hetzner-node-watchdog.fullname" .) -}}
{{- end -}}

{{- define "hetzner-node-watchdog.secretKey" -}}
{{- .Values.hcloud.existingSecretKey | default "hcloud-token" -}}
{{- end -}}
