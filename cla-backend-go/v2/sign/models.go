// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package sign

import (
	"database/sql"
	"time"
)

// DocuSignGetTokenRequest is the request body for getting a token from DocuSign
type DocuSignGetTokenRequest struct {
	GrantType string `json:"grant_type"`
	Assertion string `json:"assertion"`
}

// DocuSignGetTokenResponse is the response body for getting a token from DocuSign
type DocuSignGetTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// DocuSignUserInfoResponse is the response body for getting user info from DocuSign
type DocuSignUserInfoResponse struct {
	Sub        string `json:"sub"` // holds the GUID API username of the user that is being impersonated
	Name       string `json:"name"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Created    string `json:"created"`
	Email      string `json:"email"`
	Accounts   []struct {
		AccountId   string `json:"account_id"`
		IsDefault   bool   `json:"is_default"`
		AccountName string `json:"account_name"`
		BaseUri     string `json:"base_uri"`
	} `json:"accounts"`
}

// DocuSignEnvelopeRequest is the request body for an envelope from DocuSign, see: https://developers.docusign.com/docs/esign-rest-api/reference/envelopes/envelopes/create/
type DocuSignEnvelopeRequest struct {
	EnvelopeId           string                    `json:"envelopeId,omitempty"`           // The envelope ID of the envelope
	EnvelopeIdStamping   string                    `json:"envelopeIdStamping,omitempty"`   // When true, Envelope ID Stamping is enabled. After a document or attachment is stamped with an Envelope ID, the ID is seen by all recipients and becomes a permanent part of the document and cannot be removed.
	TemplateId           string                    `json:"templateId,omitempty"`           // The ID of the template. If a value is not provided, DocuSign generates a value.
	Documents            []DocuSignDocument        `json:"documents,omitempty"`            // A data model containing details about the documents associated with the envelope
	DocumentBase64       string                    `json:"documentBase64,omitempty"`       // The document's bytes. This field can be used to include a base64 version of the document bytes within an envelope definition instead of sending the document using a multi-part HTTP request. The maximum document size is smaller if this field is used due to the overhead of the base64 encoding.
	DocumentsCombinedUri string                    `json:"documentsCombinedUri,omitempty"` // The URI for retrieving all of the documents associated with the envelope as a single PDF file.
	DocumentsUri         string                    `json:"documentsUri,omitempty"`         // The URI for retrieving all of the documents associated with the envelope as separate files.
	EmailSubject         string                    `json:"emailSubject,omitempty"`         // EmailSubject - The subject line of the email message that is sent to all recipients.
	EmailBlurb           string                    `json:"emailBlurb,omitempty"`           // EmailBlurb - This is the same as the email body. If specified it is included in email body for all envelope recipients.
	Recipients           DocuSignRecipientType     `json:"recipients,omitempty"`
	TemplateRoles        []DocuSignTemplateRole    `json:"templateRoles,omitempty"`
	EventNotification    DocuSignEventNotification `json:"eventNotification,omitempty"`

	/* Status
	Indicates the envelope status. Valid values when creating an envelope are:

	    created: The envelope is created as a draft. It can be modified and sent later.
	    sent: The envelope will be sent to the recipients after the envelope is created.

	You can query these additional statuses once the recipients have interacted with the envelope.

	    completed: The recipients have finished working with the envelope: the documents are signed and all required tabs are filled in.
	    declined: The envelope has been declined by the recipients.
	    delivered: The envelope has been delivered to the recipients.
	    signed: The envelope has been signed by the recipients.
	    voided: The envelope is no longer valid and recipients cannot access or sign the envelope.

	*/
	Status string `json:"status,omitempty"`
}

// DocusignEnvelopeResponse
type DocusignEnvelopeResponse struct {
	EnvelopeId     string `json:"envelopeId,omitempty"`
	Status         string `json:"status,omitempty"`
	StatusDateTime string `json:"statusDateTime,omitempty"`
	Uri            string `json:"uri,omitempty"`
}

// DocuSignDocument is the data model for a document from DocuSign
type DocuSignDocument struct {
	DocumentId        string `json:"documentId,omitempty"`     // Specifies the document ID of this document. This value is used by tabs to determine which document they appear in.
	DocumentBase64    string `json:"documentBase64,omitempty"` // The document's bytes. This field can be used to include a base64 version of the document bytes within an envelope definition instead of sending the document using a multi-part HTTP request. The maximum document size is smaller if this field is used due to the overhead of the base64 encoding.0:w
	FileExtension     string `json:"fileExtension,omitempty"`  // The file extension type of the document. Non-PDF documents are converted to PDF. If the document is not a PDF, fileExtension is required. If you try to upload a non-PDF document without a fileExtension, you will receive an "unable to load document" error message. The file extension type of the document. If the document is not a PDF it is converted to a PDF.
	FileFormatHint    string `json:"fileFormatHint,omitempty"`
	IncludeInDownload string `json:"includeInDownload,omitempty"` // When set to true, the document is included in the combined document download.
	Name              string `json:"name,omitempty"`              // The name of the document. This is the name that appears in the list of documents when managing an envelope.
	Order             string `json:"order,omitempty"`             // The order in which to sort the results. Valid values are: asc, desc
}

// DocuSignRecipientType is the data model for a recipient from DocuSign
type DocuSignRecipientType struct {
	Agents              []DocuSignRecipient `json:"agent,omitempty"`
	CarbonCopies        []DocuSignRecipient `json:"carbonCopy,omitempty"`
	CertifiedDeliveries []DocuSignRecipient `json:"certifiedDelivery,omitempty"`
	Editors             []DocuSignRecipient `json:"editor,omitempty"`
	InPersonSigners     []DocuSignRecipient `json:"inPersonSigner,omitempty"`
	Intermediaries      []DocuSignRecipient `json:"intermediary,omitempty"`
	Notaries            []DocuSignRecipient `json:"notaryRecipient,omitempty"`
	Participants        []DocuSignRecipient `json:"participant,omitempty"`
	Seals               []DocuSignRecipient `json:"seals,omitempty"`          // A list of electronic seals to apply to documents.
	Signers             []DocuSignRecipient `json:"signers,omitempty"`        // A list of signers on the envelope.
	Witnesses           []DocuSignRecipient `json:"witness,omitempty"`        // A list of signers who act as witnesses for an envelope.
	RecipientCount      string              `json:"recipientCount,omitempty"` // The number of recipients in the envelope.
}

// DocuSignRecipient is the data model for an editor or signer from DocuSign
type DocuSignRecipient struct {
	RecipientId string `json:"recipientId,omitempty"` // Unique for the recipient. It is used by the tab element to indicate which recipient is to sign the document.

	ClientUserId string `json:"clientUserId,omitempty"` // Specifies whether the recipient is embedded or remote. If the clientUserId property is not null then the recipient is embedded. Use this field to associate the signer with their userId in your app. Authenticating the user is the responsibility of your app when you use embedded signing.

	/* The recipient type, as specified by the following values:
	agent:             Agent recipients can add name and email information for recipients that appear after the agent in routing order.
	carbonCopy:        Carbon copy recipients get a copy of the envelope but don't need to sign, initial, date, or add information to any of the documents. This type of recipient can be used in any routing order.
	certifiedDelivery: Certified delivery recipients must receive the completed documents for the envelope to be completed. They don't need to sign, initial, date, or add information to any of the documents.
	editor:            Editors have the same management and access rights for the envelope as the sender. Editors can add name and email information, add or change the routing order, set authentication options, and can edit signature/initial tabs and data fields for the remaining recipients.
	inPersonSigner:    In-person recipients are DocuSign users who act as signing hosts in the same physical location as the signer.
	intermediaries:    Intermediary recipients can optionally add name and email information for recipients at the same or subsequent level in the routing order.
	seal:              Electronic seal recipients represent legal entities.
	signer:            Signers are recipients who must sign, initial, date, or add data to form fields on the documents in the envelope.
	witness:           Witnesses are recipients whose signatures affirm that the identified signers have signed the documents in the envelope.
	*/
	RecipientType string `json:"recipientType,omitempty"`

	RoleName string `json:"roleName,omitempty"` // Optional element. Specifies the role name associated with the recipient. This property is required when you are working with template recipients.

	RoutingOrder string `json:"routingOrder,omitempty"` // Specifies the routing order of the recipient in the envelope.

	Name      string `json:"name,omitempty"`      // The full legal name of the recipient. Maximum Length: 100 characters. Note: You must always set a value for this property in requests, even if firstName and lastName are set.
	FirstName string `json:"firstName,omitempty"` // recipient's first name (50 characters maximum)
	LastName  string `json:"lastName,omitempty"`  // recipient's last name
	Email     string `json:"email,omitempty"`     // recipient's email address
	Note      string `json:"note,omitempty"`      // A note sent to the recipient in the signing email. This note is unique to this recipient. In the user interface, it appears near the upper left corner of the document on the signing screen. Maximum Length: 1000 characters.

	Tabs DocuSignTab `json:"tabs"` // The tabs associated with the recipient. The tabs property enables you to programmatically position tabs on the document. For example, you can specify that the SIGN_HERE tab is placed at a given (x,y) location on the document. You can also specify the font, font color, font size, and other properties of the text in the tab. You can also specify the location and size of the tab. For example, you can specify that the tab is 50 pixels wide and 20 pixels high. You can also specify the page number on which the tab is located and whether the tab is located in a document, a template, or an inline template. For more information about tabs, see the Tabs section of the REST API documentation.
}

// TextOptionalTab

type TextOptionalTab struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Height   int    `json:"height"`
	Width    int    `json:"width"`
	Locked   bool   `json:"locked"`
	Required bool   `json:"required"`
}

// DocuSignTab is the data model for a tab from DocuSign
type DocuSignTab struct {
	ApproveTabs              []DocuSignTabDetails `json:"approveTabs,omitempty"`
	CheckBoxTabs             []DocuSignTabDetails `json:"checkboxTabs,omitempty"`
	CommentThreadTabs        []DocuSignTabDetails `json:"commentThreadTabs,omitempty"`
	CommissionCountyTabs     []DocuSignTabDetails `json:"commissionCountyTabs,omitempty"`
	CommissionExpirationTabs []DocuSignTabDetails `json:"commissionExpirationTabs,omitempty"`
	CommissionNumberTabs     []DocuSignTabDetails `json:"commissionNumberTabs,omitempty"`
	CommissionStateTabs      []DocuSignTabDetails `json:"commissionStateTabs,omitempty"`
	CompanyTabs              []DocuSignTabDetails `json:"companyTabs,omitempty"`
	DateSignedTabs           []DocuSignTabDetails `json:"dateSignedTabs,omitempty"`
	DateTabs                 []DocuSignTabDetails `json:"dateTabs,omitempty"`
	DeclinedTabs             []DocuSignTabDetails `json:"declineTabs,omitempty"`
	DrawTabs                 []DocuSignTabDetails `json:"drawTabs,omitempty"`
	EmailAddressTabs         []DocuSignTabDetails `json:"emailAddressTabs,omitempty"`
	EmailTabs                []DocuSignTabDetails `json:"emailTabs,omitempty"`
	EnvelopeIdTabs           []DocuSignTabDetails `json:"envelopeIdTabs,omitempty"`
	FirstNameTabs            []DocuSignTabDetails `json:"firstNameTabs,omitempty"`
	FormulaTabs              []DocuSignTabDetails `json:"formulaTab,omitempty"`
	FullNameTabs             []DocuSignTabDetails `json:"fullNameTabs,omitempty"`
	InitialHereTabs          []DocuSignTabDetails `json:"initialHereTabs,omitempty"`
	LastNameTabs             []DocuSignTabDetails `json:"lastNameTabs,omitempty"`
	ListTabs                 []DocuSignTabDetails `json:"listTabs,omitempty"`
	NotarizeTabs             []DocuSignTabDetails `json:"notarizeTabs,omitempty"`
	NotarySealTabs           []DocuSignTabDetails `json:"notarySealTabs,omitempty"`
	NoteTabs                 []DocuSignTabDetails `json:"noteTabs,omitempty"`
	NumberTabs               []DocuSignTabDetails `json:"numberTabs,omitempty"`
	NumericalTabs            []DocuSignTabDetails `json:"numericalTabs,omitempty"`
	PhoneNumberTabs          []DocuSignTabDetails `json:"phoneNumberTabs,omitempty"`
	PolyLineOverlayTabs      []DocuSignTabDetails `json:"polyLineOverlayTabs,omitempty"`
	PrefillTabs              []DocuSignTabDetails `json:"prefillTabs,omitempty"`
	RadioGroupTabs           []DocuSignTabDetails `json:"radioGroupTabs,omitempty"`
	SignerAttachmentTabs     []DocuSignTabDetails `json:"signerAttachmentTabs,omitempty"`
	SignHereTabs             []DocuSignTabDetails `json:"signHereTabs,omitempty"`
	SmartSectionTabs         []DocuSignTabDetails `json:"smartSectionTabs,omitempty"`
	SSNTabs                  []DocuSignTabDetails `json:"ssnTabs,omitempty"`
	TabGroups                []DocuSignTabDetails `json:"tabGroupTabs,omitempty"`
	TextTabs                 []DocuSignTabDetails `json:"textTabs,omitempty"`
	TitleTabs                []DocuSignTabDetails `json:"titleTabs,omitempty"`
	ViewTabs                 []DocuSignTabDetails `json:"viewTabs,omitempty"`
	ZipTabs                  []DocuSignTabDetails `json:"zipTabs,omitempty"`
	TextOptionalTabs         []DocuSignTabDetails `json:"textOptionalTabs,omitempty"`
	SignHereOptionalTabs     []DocuSignTabDetails `json:"signHereOptionalTabs,omitempty"`
}

// DocuSignTabDetails is the data model for a tab from DocuSign
type DocuSignTabDetails struct {
	AnchorCaseSensitive       string `json:"anchorCaseSensitive,omitempty"`       // anchor case sensitive flag, "true" or "false"
	AnchorIgnoreIfNotPresent  string `json:"anchorIgnoreIfNotPresent,omitempty"`  // When true, this tab is ignored if the anchorString is not found in the document.
	AnchorHorizontalAlignment string `json:"anchorHorizontalAlignment,omitempty"` // This property controls how anchor tabs are aligned in relation to the anchor text. Possible values are : left: Aligns the left side of the tab with the beginning of the first character of the matching anchor word. This is the default value. right: Aligns the tab’s left side with the last character of the matching anchor word.
	AnchorMatchWholeWord      string `json:"anchorMatchWholeWord,omitempty"`      // When true, the text string in a document must match the value of the anchorString property in its entirety for an anchor tab to be created. The default value is false. For example, when set to true, if the input is man then man will match but manpower, fireman, and penmanship will not. When false, if the input is man then man, manpower, fireman, and penmanship will all match.
	AnchorString              string `json:"anchorString,omitempty"`              // Specifies the string to find in the document and use as the basis for tab placement
	AnchorUnits               string `json:"anchorUnits,omitempty"`               // anchor units, pixels, cms, mms
	AnchorXOffset             string `json:"anchorXOffset,omitempty"`             // anchor x offset
	AnchorYOffset             string `json:"anchorYOffset,omitempty"`             // anchor y offset
	Bold                      string `json:"bold,omitempty"`                      // bold flag, "true" or "false"
	DocumentId                string `json:"documentId,omitempty"`                // Specifies the document ID number that the tab is placed on. This must refer to an existing Document's ID attribute.
	Font                      string `json:"font,omitempty"`                      // font
	FontSize                  string `json:"fontSize,omitempty"`                  // font size
	Height                    string `json:"height,omitempty"`                    // The height of the tab in pixels. Must be an integer.
	Locked                    string `json:"locked,omitempty"`                    // locked flag, "true" or "false"
	MinNumericalValue         string `json:"minNumericalValue,omitempty"`         // minimum numerical value, such as "0", used for validation of numerical tabs
	MaxNumericalValue         string `json:"maxNumericalValue,omitempty"`         // maximum numerical value, such as "100", used for validation of numerical tabs
	Name                      string `json:"name,omitempty"`                      // The name of the tab. For example, Sign Here or Initial Here. If the tooltip attribute is not set, this value will be displayed as the custom tooltip text.
	Optional                  string `json:"optional,omitempty"`                  // When true, the recipient does not need to complete this tab to complete the signing process
	PageNumber                string `json:"pageNumber,omitempty"`                // Specifies the page number on which the tab is located. Must be 1 for supplemental documents.
	Required                  string `json:"required,omitempty"`                  // When true, the signer is required to fill out this tab
	TabId                     string `json:"tabId,omitempty"`                     // tab idj
	TabLabel                  string `json:"tabLabel,omitempty"`                  // label
	TabOrder                  string `json:"tabOrder,omitempty"`                  // A positive integer that sets the order the tab is navigated to during signing. Tabs on a page are navigated to in ascending order, starting with the lowest number and moving to the highest. If two or more tabs have the same tabOrder value, the normal auto-navigation setting behavior for the envelope is used.
	TabType                   string `json:"tabType,omitempty"`                   // Indicates type of tab (for example: signHere or initialHere)
	ToolTip                   string `json:"toolTip,omitempty"`                   // The text of a tooltip that appears when a user hovers over a form field or tab.
	Width                     string `json:"width,omitempty"`                     // The width of the tab in pixels. Must be an integer. This is not applicable to Sign Here tab.
	XPosition                 string `json:"xPosition,omitempty"`                 // x position
	YPosition                 string `json:"yPosition,omitempty"`                 // x position
	ValidationType            string `json:"validationType,omitempty"`            // validation type, "string", "number", "date", "zipcode", "currency"
	Value                     string `json:"value,omitempty"`
	CustomTabId               string `json:"customTabId,omitempty"`
}

// DocuSignTemplateRole is the request body for a template role from DocuSign
type DocuSignTemplateRole struct {
	Name         string `json:"name,omitempty"`         // the recipient's email address
	Email        string `json:"email,omitempty"`        // the recipient's name
	RoleName     string `json:"roleName,omitempty"`     // the template role name associated with the recipient
	ClientUserID string `json:"clientUserId,omitempty"` // Specifies whether the recipient is embedded or remote. If the clientUserId property is not null then the recipient is embedded. Use this field to associate the signer with their userId in your app. Authenticating the user is the responsibility of your app when you use embedded signing. If the clientUserId property is set and either SignerMustHaveAccount or SignerMustLoginToSign property of the account settings is set to true, an error is generated on sending.
	RoutingOrder string `json:"routingOrder,omitempty"` // Specifies the routing order of the recipient in the envelope.
}

// DocuSignEnvelopeResponse is the response body for an envelope from DocuSign, see: https://developers.docusign.com/docs/esign-rest-api/reference/envelopes/envelopes/update/
type DocuSignEnvelopeResponse struct {
	EnvelopeId   string              `json:"envelopeId,omitempty"`
	Recipients   []DocuSignRecipient `json:"recipients,omitempty"`
	ErrorDetails struct {
		ErrorCode string `json:"errorCode,omitempty"`
		Message   string `json:"message,omitempty"`
	} `json:"errorDetails,omitempty"`
}

// DocuSignEnvelopeResponseModel is the envelope response model
type DocuSignEnvelopeResponseModel struct {
	/*
		// Response from: https://developers.docusign.com/docs/esign-rest-api/reference/envelopes/envelopes/get/
			{
			  "allowMarkup": "false",
			  "autoNavigation": "true",
			  "brandId": "56502fe1-xxxx-xxxx-xxxx-97cb5c43176a",
			  "certificateUri": "/envelopes/4b728be4-xxxx-xxxx-xxxx-d63e23f822b6/documents/certificate",
			  "createdDateTime": "2016-10-05T01:04:58.1830000Z",
			  "customFieldsUri": "/envelopes/4b728be4-xxxx-xxxx-xxxx-d63e23f822b6/custom_fields",
			  "documentsCombinedUri": "/envelopes/4b728be4-xxxx-xxxx-xxxx-d63e23f822b6/documents/combined",
			  "documentsUri": "/envelopes/4b728be4-xxxx-xxxx-xxxx-d63e23f822b6/documents",
			  "emailSubject": "Please sign the NDA",
			  "enableWetSign": "true",
			  "envelopeId": "4b728be4-xxxx-xxxx-xxxx-d63e23f822b6",
			  "envelopeIdStamping": "true",
			  "envelopeUri": "/envelopes/4b728be4-xxxx-xxxx-xxxx-d63e23f822b6",
			  "initialSentDateTime": "2016-10-05T01:04:58.7770000Z",
			  "is21CFRPart11": "false",
			  "isSignatureProviderEnvelope": "false",
			  "lastModifiedDateTime": "2016-10-05T01:04:58.1830000Z",
			  "notificationUri": "/envelopes/4b728be4-xxxx-xxxx-xxxx-d63e23f822b6/notification",
			  "purgeState": "unpurged",
			  "recipientsUri": "/envelopes/4b728be4-xxxx-xxxx-xxxx-d63e23f822b6/recipients",
			  "sentDateTime": "2016-10-05T01:04:58.7770000Z",
			  "status": "sent",
			  "statusChangedDateTime": "2016-10-05T01:04:58.7770000Z",
			  "templatesUri": "/envelopes/4b728be4-xxxx-xxxx-xxxx-d63e23f822b6/templates"
			}
	*/
	AllowMarkup                 string `json:"allowMarkup,omitempty"`
	AutoNavigation              string `json:"autoNavigation,omitempty"`
	BrandId                     string `json:"brandId,omitempty"`
	CertificateUri              string `json:"certificateUri,omitempty"`
	CreatedDateTime             string `json:"createdDateTime,omitempty"`
	CustomFieldsUri             string `json:"customFieldsUri,omitempty"`
	DocumentsCombinedUri        string `json:"documentsCombinedUri,omitempty"`
	DocumentsUri                string `json:"documentsUri,omitempty"`
	EmailSubject                string `json:"emailSubject,omitempty"`
	EnableWetSign               string `json:"enableWetSign,omitempty"`
	EnvelopeId                  string `json:"envelopeId,omitempty"`
	EnvelopeIdStamping          string `json:"envelopeIdStamping,omitempty"`
	EnvelopeUri                 string `json:"envelopeUri,omitempty"`
	InitialSentDateTime         string `json:"initialSentDateTime,omitempty"`
	Is21CFRPart11               string `json:"is21CFRPart11,omitempty"`
	IsSignatureProviderEnvelope string `json:"isSignatureProviderEnvelope,omitempty"`
	LastModifiedDateTime        string `json:"lastModifiedDateTime,omitempty"`
	NotificationUri             string `json:"notificationUri,omitempty"`
	PurgeState                  string `json:"purgeState,omitempty"`
	RecipientsUri               string `json:"recipientsUri,omitempty"`
	SentDateTime                string `json:"sentDateTime,omitempty"`
	Status                      string `json:"status,omitempty"`
	StatusChangedDateTime       string `json:"statusChangedDateTime,omitempty"`
	TemplatesUri                string `json:"templatesUri,omitempty"`
}

// IndividualMembershipDocuSignDBSummaryModel is the data model for an individual membership DocuSign database summary models
type IndividualMembershipDocuSignDBSummaryModel struct {
	DocuSignEnvelopeID               string         `db:"docusign_envelope_id"`
	DocuSignEnvelopeCreatedAt        string         `db:"docusign_envelope_created_at"`
	DocuSignEnvelopeSigningStatus    string         `db:"docusign_envelope_signing_status"`
	DocuSignEnvelopeSigningUpdatedAt time.Time      `db:"docusign_envelope_signing_updated_at"`
	Memo                             sql.NullString `db:"memo"`
	//DocuSignEnvelopeSignedDate       time.Time `json:"docusign_envelope_signed_date"`
}

type ClaSignatoryEmailParams struct {
	ClaGroupName    string
	SignatoryName   string
	ClaManagerName  string
	ClaManagerEmail string
	CompanyName     string
	ProjectVersion  string
	ProjectNames    []string
}

type DocuSignRecipientEvent struct {
	EnvelopeEventStatusCode string `json:"envelopeEventStatusCode"`
}

type DocuSignEventNotification struct {
	URL            string                   `json:"url"`
	LoggingEnabled bool                     `json:"loggingEnabled"`
	EnvelopeEvents []DocuSignRecipientEvent `json:"envelopeEvents"`
}

type Recipient struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	// Other recipient-specific fields
}

// DocuSignUpdateDocumentResponse is the response body for adding/updating a document to an envelope from DocuSign
type DocuSignUpdateDocumentResponse struct {
	/*
		{"documentId":"1","documentIdGuid":"2c205f31-4c6b-4237-b6bc-d79457b949a5","name":"document.pdf","type":"content","uri":"/envelopes/ebeee6a6-c17f-4d05-8441-38d5c1ad9675/documents/1","order":"1","containsPdfFormFields":"false","templateRequired":"false","authoritativeCopy":"false"}
	*/
	DocumentId            string `json:"documentId,omitempty"`
	DocumentIdGuid        string `json:"documentIdGuid,omitempty"`
	Name                  string `json:"name,omitempty"`
	Type                  string `json:"type,omitempty"`
	Uri                   string `json:"uri,omitempty"`
	Order                 string `json:"order,omitempty"`
	ContainsPdfFormFields string `json:"containsPdfFormFields,omitempty"`
	TemplateRequired      string `json:"templateRequired,omitempty"`
	AuthoritativeCopy     string `json:"authoritativeCopy,omitempty"`
}

type Signer struct {
	CreationReason  string `json:"creationReason"`
	IsBulkRecipient string `json:"isBulkRecipient"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	RecipientId     string `json:"recipientId"`
	RecipientIdGuid string `json:"recipientIdGuid"`
	RequireIdLookup string `json:"requireIdLookup"`
	UserId          string `json:"userId"`
	ClientUserId    string `json:"clientUserId"`
	RoutingOrder    string `json:"routingOrder"`
	RoleName        string `json:"roleName"`
	Status          string `json:"status"`
}

type DocusignRecipientResponse struct {
	Signers []Signer `json:"signers"`
}

type DocusignRecipientView struct {
	Email               string `json:"email"`
	Username            string `json:"userName"`
	ReturnURL           string `json:"returnUrl"`
	RecipientID         string `json:"recipientId"`
	ClientUserId        string `json:"clientUserId,omitempty"`
	AuthenticaionMethod string `json:"authenticationMethod"`
}

type DocusignRecipientViewResponse struct {
	URL string `json:"url"`
}
