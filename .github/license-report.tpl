{{- range . }}
Package: {{ .Name }}
License: {{ .LicenseName }}
License URL: {{ .LicenseURL }}
---
{{- end }}
