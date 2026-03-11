package contracts

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed templates/*
var templatesFS embed.FS

var (
	templatesOnce sync.Once
	templatesMap  map[string]TemplateConfig
	templatesErr  error
)

func loadTemplates() error {
	templatesOnce.Do(func() {
		b, err := templatesFS.ReadFile("templates/templates.json")
		if err != nil {
			templatesErr = fmt.Errorf("read templates.json: %w", err)
			return
		}
		var m map[string]TemplateConfig
		if err := json.Unmarshal(b, &m); err != nil {
			templatesErr = fmt.Errorf("unmarshal templates.json: %w", err)
			return
		}
		templatesMap = m
	})
	return templatesErr
}

// Get returns a template definition by name.
func Get(name string) (TemplateConfig, bool, error) {
	if err := loadTemplates(); err != nil {
		return TemplateConfig{}, false, err
	}
	cfg, ok := templatesMap[name]
	return cfg, ok, nil
}

// RenderHTML renders a contract HTML file for the given template name and documentType.
//
// documentType should be "individual" or "corporate" (case-insensitive).
//
// This mirrors cla.resources.contract_templates.ContractTemplate.get_html_contract.
func RenderHTML(templateName, documentType string, major, minor int, legalEntityName, preamble string) (string, error) {
	cfg, ok, err := Get(templateName)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("unknown template: %s", templateName)
	}

	dt := normalizeDocumentType(documentType)
	fname, ok := cfg.Files[dt]
	if !ok || fname == "" {
		return "", fmt.Errorf("template %s missing file for documentType %s", templateName, dt)
	}

	b, err := templatesFS.ReadFile("templates/" + fname)
	if err != nil {
		return "", fmt.Errorf("read template html %s: %w", fname, err)
	}
	html := string(b)

	// Placeholder replacements match the legacy Python implementation.
	html = strings.ReplaceAll(html, "{{document_type}}", dt)
	html = strings.ReplaceAll(html, "{{major_version}}", fmt.Sprintf("%d", major))
	html = strings.ReplaceAll(html, "{{minor_version}}", fmt.Sprintf("%d", minor))
	html = strings.ReplaceAll(html, "{{legal_entity_name}}", legalEntityName)
	html = strings.ReplaceAll(html, "{{preamble}}", preamble)

	return html, nil
}

// Tabs returns the template's raw tab definitions for the given documentType.
//
// documentType should be "individual" or "corporate" (case-insensitive).
func Tabs(templateName, documentType string) ([]TabData, error) {
	cfg, ok, err := Get(templateName)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("unknown template: %s", templateName)
	}
	dt := normalizeDocumentType(documentType)
	tabs, ok := cfg.Tabs[dt]
	if !ok {
		return nil, fmt.Errorf("template %s missing tabs for documentType %s", templateName, dt)
	}
	// Return a shallow copy to avoid accidental mutation by callers.
	out := make([]TabData, len(tabs))
	copy(out, tabs)
	return out, nil
}

func normalizeDocumentType(documentType string) string {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "corporate", "ccla":
		return "Corporate"
	default:
		// Python defaults to Individual.
		return "Individual"
	}
}
