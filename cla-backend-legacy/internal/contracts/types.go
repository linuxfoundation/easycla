// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package contracts

// TabData matches the dicts returned by the legacy Python cla.resources.contract_templates.*.get_tabs().
//
// These definitions are converted into persisted DynamoDB document_tabs entries (DocumentTabModel)
// by handlers when creating a project document template.
type TabData struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`

	AnchorString             string `json:"anchor_string,omitempty"`
	AnchorIgnoreIfNotPresent string `json:"anchor_ignore_if_not_present,omitempty"`
	AnchorXOffset            int    `json:"anchor_x_offset,omitempty"`
	AnchorYOffset            int    `json:"anchor_y_offset,omitempty"`

	// Absolute positioning (used by some templates).
	PositionX int `json:"position_x,omitempty"`
	PositionY int `json:"position_y,omitempty"`

	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
	Page   int `json:"page,omitempty"`
}

// TemplateConfig is loaded from templates/templates.json.
type TemplateConfig struct {
	Prefix string               `json:"prefix"`
	Files  map[string]string    `json:"files"`
	Tabs   map[string][]TabData `json:"tabs"`
}
