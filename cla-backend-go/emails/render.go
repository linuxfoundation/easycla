// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"bytes"
	"html/template"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

// RenderTemplate renders the template for given template with given params
func RenderTemplate(claGroupVersion, templateName, templateStr string, params interface{}) (string, error) {
	tmpl := template.New(templateName)
	t, err := tmpl.Parse(templateStr)
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	if err := t.Execute(&tpl, params); err != nil {
		return "", err
	}

	result := tpl.String()
	result = result + utils.GetEmailHelpContent(claGroupVersion == utils.V2)
	result = result + utils.GetEmailSignOffContent()
	return result, nil
}
