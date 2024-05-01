# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

import xml.etree.ElementTree as ET

from cla.models.docusign_models import (ClaSignatoryEmailParams,
                                        cla_signatory_email_content,
                                        create_default_company_values,
                                        document_signed_email_content,
                                        populate_signature_from_ccla_callback,
                                        populate_signature_from_icla_callback)
from cla.models.dynamo_models import Company, Project, Signature, User

content_icla_agreement_date = """<?xml version="1.0" encoding="utf-8"?>
<DocuSignEnvelopeInformation xmlns:xsd="http://www.w3.org/2001/XMLSchema"
                             xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
                             xmlns="http://www.docusign.net/API/3.0">
    <EnvelopeStatus>
        <RecipientStatuses>
            <RecipientStatus>
                <Type>Signer</Type>
                <Email>example@example.org</Email>
                <UserName>Unknown</UserName>
                <RoutingOrder>1</RoutingOrder>
                <Sent>2020-12-21T08:29:09.947</Sent>
                <Delivered>2020-12-21T08:29:20.527</Delivered>
                <Signed>2020-12-21T08:30:10.133</Signed>
                <DeclineReason xsi:nil="true"/>
                <Status>Completed</Status>
                <RecipientIPAddress>95.87.31.3</RecipientIPAddress>
                <ClientUserId>9da896e1-c44a-4304-900f-933f27018a27</ClientUserId>
                <CustomFields/>
                <TabStatuses>
                    <TabStatus>
                        <TabType>SignHere</TabType>
                        <Status>Signed</Status>
                        <XPosition>233</XPosition>
                        <YPosition>22</YPosition>
                        <TabLabel>sign</TabLabel>
                        <TabName>Please Sign</TabName>
                        <TabValue/>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>218</XPosition>
                        <YPosition>166</YPosition>
                        <TabLabel>full_name</TabLabel>
                        <TabName>Full Name</TabName>
                        <TabValue>Example FullName</TabValue>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>281</XPosition>
                        <YPosition>227</YPosition>
                        <TabLabel>mailing_address1</TabLabel>
                        <TabName>Mailing Address</TabName>
                        <TabValue>asdf</TabValue>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>116</XPosition>
                        <YPosition>277</YPosition>
                        <TabLabel>mailing_address2</TabLabel>
                        <TabName>Mailing Address</TabName>
                        <TabValue>asdf</TabValue>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>116</XPosition>
                        <YPosition>331</YPosition>
                        <TabLabel>mailing_address3</TabLabel>
                        <TabName>Mailing Address</TabName>
                        <TabValue>asdf</TabValue>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>210</XPosition>
                        <YPosition>400</YPosition>
                        <TabLabel>country</TabLabel>
                        <TabName>Country</TabName>
                        <TabValue>Bulgaria</TabValue>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>195</XPosition>
                        <YPosition>456</YPosition>
                        <TabLabel>email</TabLabel>
                        <TabName>Email</TabName>
                        <TabValue>example@example.com</TabValue>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                        <OriginalValue>example@example.org</OriginalValue>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>DateSigned</TabType>
                        <Status>Signed</Status>
                        <XPosition>735</XPosition>
                        <YPosition>110</YPosition>
                        <TabLabel>date</TabLabel>
                        <TabName>Date</TabName>
                        <TabValue>12/21/2020</TabValue>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                    </TabStatus>
                </TabStatuses>
                <RecipientAttachment>
                    <Attachment>
                        <Data>
                            PEZvcm1EYXRhPjx4ZmRmPjxmaWVsZHM+PGZpZWxkIG5hbWU9ImZ1bGxfbmFtZSI+PHZhbHVlPkRlbmlzIEs8L3ZhbHVlPjwvZmllbGQ+PGZpZWxkIG5hbWU9Im1haWxpbmdfYWRkcmVzczEiPjx2YWx1ZT5NaXIgOTwvdmFsdWU+PC9maWVsZD48ZmllbGQgbmFtZT0ibWFpbGluZ19hZGRyZXNzMiI+PHZhbHVlPlNoZXlub3ZvPC92YWx1ZT48L2ZpZWxkPjxmaWVsZCBuYW1lPSJtYWlsaW5nX2FkZHJlc3MzIj48dmFsdWU+S2F6YW5sYWs8L3ZhbHVlPjwvZmllbGQ+PGZpZWxkIG5hbWU9ImNvdW50cnkiPjx2YWx1ZT5CdWxnYXJpYTwvdmFsdWU+PC9maWVsZD48ZmllbGQgbmFtZT0iZW1haWwiPjx2YWx1ZT5tYWtrYWxvdEBnbWFpbC5jb208L3ZhbHVlPjwvZmllbGQ+PGZpZWxkIG5hbWU9IkRhdGVTaWduZWQiPjx2YWx1ZT4xMi8yMS8yMDIwPC92YWx1ZT48L2ZpZWxkPjwvZmllbGRzPjwveGZkZj48L0Zvcm1EYXRhPg==
                        </Data>
                        <Label>DSXForm</Label>
                    </Attachment>
                </RecipientAttachment>
                <AccountStatus>Active</AccountStatus>
                <EsignAgreementInformation>
                    <AccountEsignId>f78e337a-a9c7-47e6-bc20-6f75a84706ba</AccountEsignId>
                    <UserEsignId>81225123-b7a0-4650-afd4-2e27d8017e8b</UserEsignId>
                    <AgreementDate>2020-12-21T08:29:20.51</AgreementDate>
                </EsignAgreementInformation>
                <FormData>
                    <xfdf>
                        <fields>
                            <field name="full_name">
                                <value>Example FullName</value>
                            </field>
                            <field name="mailing_address1">
                                <value>asdf</value>
                            </field>
                            <field name="mailing_address2">
                                <value>asdf</value>
                            </field>
                            <field name="mailing_address3">
                                <value>asdf</value>
                            </field>
                            <field name="country">
                                <value>Bulgaria</value>
                            </field>
                            <field name="email">
                                <value>example@example.com</value>
                            </field>
                            <field name="DateSigned">
                                <value>12/21/2020</value>
                            </field>
                        </fields>
                    </xfdf>
                </FormData>
                <RecipientId>34dc5447-2f10-4334-8fea-f94b500e7202</RecipientId>
            </RecipientStatus>
        </RecipientStatuses>
        <TimeGenerated>2020-12-21T08:30:37.9661043</TimeGenerated>
        <EnvelopeID>c5c02f0b-d66b-4ad5-950d-0319ed3e1473</EnvelopeID>
        <Subject>EasyCLA: CLA Signature Request for aswf-signatory-name-test</Subject>
        <UserName>Example FullName</UserName>
        <Email>example@example.com</Email>
        <Status>Completed</Status>
        <Created>2020-12-21T08:29:09.383</Created>
        <Sent>2020-12-21T08:29:09.977</Sent>
        <Delivered>2020-12-21T08:29:20.793</Delivered>
        <Signed>2020-12-21T08:30:10.133</Signed>
        <Completed>2020-12-21T08:30:10.133</Completed>
        <ACStatus>Original</ACStatus>
        <ACStatusDate>2020-12-21T08:29:09.383</ACStatusDate>
        <ACHolder>Example FullName</ACHolder>
        <ACHolderEmail>example@example.com</ACHolderEmail>
        <ACHolderLocation>DocuSign</ACHolderLocation>
        <SigningLocation>Online</SigningLocation>
        <SenderIPAddress>54.80.186.114</SenderIPAddress>
        <EnvelopePDFHash/>
        <CustomFields>
            <CustomField>
                <Name>AccountId</Name>
                <Show>false</Show>
                <Required>false</Required>
                <Value>10406522</Value>
                <CustomFieldType>Text</CustomFieldType>
            </CustomField>
            <CustomField>
                <Name>AccountName</Name>
                <Show>false</Show>
                <Required>false</Required>
                <Value>Linux Foundation</Value>
                <CustomFieldType>Text</CustomFieldType>
            </CustomField>
            <CustomField>
                <Name>AccountSite</Name>
                <Show>false</Show>
                <Required>false</Required>
                <Value>demo</Value>
                <CustomFieldType>Text</CustomFieldType>
            </CustomField>
        </CustomFields>
        <AutoNavigation>true</AutoNavigation>
        <EnvelopeIdStamping>true</EnvelopeIdStamping>
        <AuthoritativeCopy>false</AuthoritativeCopy>
        <DocumentStatuses>
            <DocumentStatus>
                <ID>22317</ID>
                <Name>ASWF 2020 v2.1</Name>
                <TemplateName/>
                <Sequence>1</Sequence>
            </DocumentStatus>
        </DocumentStatuses>
    </EnvelopeStatus>
    <TimeZone>Pacific Standard Time</TimeZone>
    <TimeZoneOffset>-8</TimeZoneOffset>
</DocuSignEnvelopeInformation>
"""
content_icla_missing_agreement_date = """<?xml version="1.0" encoding="utf-8"?>
<DocuSignEnvelopeInformation xmlns:xsd="http://www.w3.org/2001/XMLSchema"
                             xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
                             xmlns="http://www.docusign.net/API/3.0">
    <EnvelopeStatus>
        <RecipientStatuses>
            <RecipientStatus>
                <Type>Signer</Type>
                <Email>example@example.org</Email>
                <UserName>Unknown</UserName>
                <RoutingOrder>1</RoutingOrder>
                <Sent>2020-12-21T08:29:09.947</Sent>
                <Delivered>2020-12-21T08:29:20.527</Delivered>
                <Signed>2020-12-21T08:30:10.133</Signed>
                <DeclineReason xsi:nil="true"/>
                <Status>Completed</Status>
                <RecipientIPAddress>95.87.31.3</RecipientIPAddress>
                <ClientUserId>9da896e1-c44a-4304-900f-933f27018a27</ClientUserId>
                <CustomFields/>
                <TabStatuses>
                    <TabStatus>
                        <TabType>SignHere</TabType>
                        <Status>Signed</Status>
                        <XPosition>233</XPosition>
                        <YPosition>22</YPosition>
                        <TabLabel>sign</TabLabel>
                        <TabName>Please Sign</TabName>
                        <TabValue/>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>218</XPosition>
                        <YPosition>166</YPosition>
                        <TabLabel>full_name</TabLabel>
                        <TabName>Full Name</TabName>
                        <TabValue>Example FullName</TabValue>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>281</XPosition>
                        <YPosition>227</YPosition>
                        <TabLabel>mailing_address1</TabLabel>
                        <TabName>Mailing Address</TabName>
                        <TabValue>asdf</TabValue>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>116</XPosition>
                        <YPosition>277</YPosition>
                        <TabLabel>mailing_address2</TabLabel>
                        <TabName>Mailing Address</TabName>
                        <TabValue>asdf</TabValue>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>116</XPosition>
                        <YPosition>331</YPosition>
                        <TabLabel>mailing_address3</TabLabel>
                        <TabName>Mailing Address</TabName>
                        <TabValue>asdf</TabValue>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>210</XPosition>
                        <YPosition>400</YPosition>
                        <TabLabel>country</TabLabel>
                        <TabName>Country</TabName>
                        <TabValue>Bulgaria</TabValue>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>195</XPosition>
                        <YPosition>456</YPosition>
                        <TabLabel>email</TabLabel>
                        <TabName>Email</TabName>
                        <TabValue>example@example.com</TabValue>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                        <OriginalValue>example@example.org</OriginalValue>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>DateSigned</TabType>
                        <Status>Signed</Status>
                        <XPosition>735</XPosition>
                        <YPosition>110</YPosition>
                        <TabLabel>date</TabLabel>
                        <TabName>Date</TabName>
                        <TabValue>12/21/2020</TabValue>
                        <DocumentID>22317</DocumentID>
                        <PageNumber>3</PageNumber>
                    </TabStatus>
                </TabStatuses>
                <RecipientAttachment>
                    <Attachment>
                        <Data>
                            PEZvcm1EYXRhPjx4ZmRmPjxmaWVsZHM+PGZpZWxkIG5hbWU9ImZ1bGxfbmFtZSI+PHZhbHVlPkRlbmlzIEs8L3ZhbHVlPjwvZmllbGQ+PGZpZWxkIG5hbWU9Im1haWxpbmdfYWRkcmVzczEiPjx2YWx1ZT5NaXIgOTwvdmFsdWU+PC9maWVsZD48ZmllbGQgbmFtZT0ibWFpbGluZ19hZGRyZXNzMiI+PHZhbHVlPlNoZXlub3ZvPC92YWx1ZT48L2ZpZWxkPjxmaWVsZCBuYW1lPSJtYWlsaW5nX2FkZHJlc3MzIj48dmFsdWU+S2F6YW5sYWs8L3ZhbHVlPjwvZmllbGQ+PGZpZWxkIG5hbWU9ImNvdW50cnkiPjx2YWx1ZT5CdWxnYXJpYTwvdmFsdWU+PC9maWVsZD48ZmllbGQgbmFtZT0iZW1haWwiPjx2YWx1ZT5tYWtrYWxvdEBnbWFpbC5jb208L3ZhbHVlPjwvZmllbGQ+PGZpZWxkIG5hbWU9IkRhdGVTaWduZWQiPjx2YWx1ZT4xMi8yMS8yMDIwPC92YWx1ZT48L2ZpZWxkPjwvZmllbGRzPjwveGZkZj48L0Zvcm1EYXRhPg==
                        </Data>
                        <Label>DSXForm</Label>
                    </Attachment>
                </RecipientAttachment>
                <AccountStatus>Active</AccountStatus>
                <FormData>
                    <xfdf>
                        <fields>
                            <field name="full_name">
                                <value>Example FullName</value>
                            </field>
                            <field name="mailing_address1">
                                <value>asdf</value>
                            </field>
                            <field name="mailing_address2">
                                <value>asdf</value>
                            </field>
                            <field name="mailing_address3">
                                <value>asdf</value>
                            </field>
                            <field name="country">
                                <value>Bulgaria</value>
                            </field>
                            <field name="email">
                                <value>example@example.com</value>
                            </field>
                            <field name="DateSigned">
                                <value>12/21/2020</value>
                            </field>
                        </fields>
                    </xfdf>
                </FormData>
                <RecipientId>34dc5447-2f10-4334-8fea-f94b500e7202</RecipientId>
            </RecipientStatus>
        </RecipientStatuses>
        <TimeGenerated>2020-12-21T08:30:37.9661043</TimeGenerated>
        <EnvelopeID>c5c02f0b-d66b-4ad5-950d-0319ed3e1473</EnvelopeID>
        <Subject>EasyCLA: CLA Signature Request for aswf-signatory-name-test</Subject>
        <UserName>Example FullName</UserName>
        <Email>example@example.com</Email>
        <Status>Completed</Status>
        <Created>2020-12-21T08:29:09.383</Created>
        <Sent>2020-12-21T08:29:09.977</Sent>
        <Delivered>2020-12-21T08:29:20.793</Delivered>
        <Signed>2020-12-21T08:30:10.133</Signed>
        <Completed>2020-12-21T08:30:10.133</Completed>
        <ACStatus>Original</ACStatus>
        <ACStatusDate>2020-12-21T08:29:09.383</ACStatusDate>
        <ACHolder>Example FullName</ACHolder>
        <ACHolderEmail>example@example.com</ACHolderEmail>
        <ACHolderLocation>DocuSign</ACHolderLocation>
        <SigningLocation>Online</SigningLocation>
        <SenderIPAddress>54.80.186.114</SenderIPAddress>
        <EnvelopePDFHash/>
        <CustomFields>
            <CustomField>
                <Name>AccountId</Name>
                <Show>false</Show>
                <Required>false</Required>
                <Value>10406522</Value>
                <CustomFieldType>Text</CustomFieldType>
            </CustomField>
            <CustomField>
                <Name>AccountName</Name>
                <Show>false</Show>
                <Required>false</Required>
                <Value>Linux Foundation</Value>
                <CustomFieldType>Text</CustomFieldType>
            </CustomField>
            <CustomField>
                <Name>AccountSite</Name>
                <Show>false</Show>
                <Required>false</Required>
                <Value>demo</Value>
                <CustomFieldType>Text</CustomFieldType>
            </CustomField>
        </CustomFields>
        <AutoNavigation>true</AutoNavigation>
        <EnvelopeIdStamping>true</EnvelopeIdStamping>
        <AuthoritativeCopy>false</AuthoritativeCopy>
        <DocumentStatuses>
            <DocumentStatus>
                <ID>22317</ID>
                <Name>ASWF 2020 v2.1</Name>
                <TemplateName/>
                <Sequence>1</Sequence>
            </DocumentStatus>
        </DocumentStatuses>
    </EnvelopeStatus>
    <TimeZone>Pacific Standard Time</TimeZone>
    <TimeZoneOffset>-8</TimeZoneOffset>
</DocuSignEnvelopeInformation>
"""


def test_populate_signature_from_ccla_callback():
    content = """<?xml version="1.0" encoding="utf-8"?>
<DocuSignEnvelopeInformation xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
                             xmlns="http://www.docusign.net/API/3.0">
    <EnvelopeStatus>
        <RecipientStatuses>
            <RecipientStatus>
                <Type>Signer</Type>
                <Email>example@example.org</Email>
                <UserName>Example Username</UserName>
                <RoutingOrder>1</RoutingOrder>
                <Sent>2020-12-17T07:43:56.203</Sent>
                <Delivered>2020-12-17T07:44:08.52</Delivered>
                <Signed>2020-12-17T07:44:30.673</Signed>
                <DeclineReason xsi:nil="true"/>
                <Status>Completed</Status>
                <RecipientIPAddress>108.168.239.94</RecipientIPAddress>
                <ClientUserId>74dd08c2-9c4b-41ee-b65f-cf243abf65e6</ClientUserId>
                <CustomFields/>
                <TabStatuses>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>304</XPosition>
                        <YPosition>170</YPosition>
                        <TabLabel>signatory_name</TabLabel>
                        <TabName>Signatory Name</TabName>
                        <TabValue>Example Signatory</TabValue>
                        <DocumentID>47977</DocumentID>
                        <PageNumber>3</PageNumber>
                        <OriginalValue>Example Signatory</OriginalValue>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>304</XPosition>
                        <YPosition>229</YPosition>
                        <TabLabel>signatory_email</TabLabel>
                        <TabName>Signatory E-mail</TabName>
                        <TabValue>example@example.org</TabValue>
                        <DocumentID>47977</DocumentID>
                        <PageNumber>3</PageNumber>
                        <OriginalValue>example@example.org</OriginalValue>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>320</XPosition>
                        <YPosition>343</YPosition>
                        <TabLabel>corporation_name</TabLabel>
                        <TabName>Corporation Name</TabName>
                        <TabValue>The Linux Foundation</TabValue>
                        <DocumentID>47977</DocumentID>
                        <PageNumber>3</PageNumber>
                        <OriginalValue>The Linux Foundation</OriginalValue>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>412</XPosition>
                        <YPosition>575</YPosition>
                        <TabLabel>cla_manager_name</TabLabel>
                        <TabName>Initial CLA Manager Name</TabName>
                        <TabValue>Example Signatory</TabValue>
                        <DocumentID>47977</DocumentID>
                        <PageNumber>3</PageNumber>
                        <OriginalValue>Example Signatory</OriginalValue>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>412</XPosition>
                        <YPosition>631</YPosition>
                        <TabLabel>cla_manager_email</TabLabel>
                        <TabName>Initial CLA Manager Email</TabName>
                        <TabValue>example@example.org</TabValue>
                        <DocumentID>47977</DocumentID>
                        <PageNumber>3</PageNumber>
                        <OriginalValue>example@example.org</OriginalValue>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>SignHere</TabType>
                        <Status>Signed</Status>
                        <XPosition>264</XPosition>
                        <YPosition>22</YPosition>
                        <TabLabel>sign</TabLabel>
                        <TabName>Please Sign</TabName>
                        <TabValue/>
                        <DocumentID>47977</DocumentID>
                        <PageNumber>3</PageNumber>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>304</XPosition>
                        <YPosition>285</YPosition>
                        <TabLabel>signatory_title</TabLabel>
                        <TabName>Signatory Title</TabName>
                        <TabValue>CEO</TabValue>
                        <DocumentID>47977</DocumentID>
                        <PageNumber>3</PageNumber>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>327</XPosition>
                        <YPosition>397</YPosition>
                        <TabLabel>corporation_address1</TabLabel>
                        <TabName>Corporation Address1</TabName>
                        <TabValue>113</TabValue>
                        <DocumentID>47977</DocumentID>
                        <PageNumber>3</PageNumber>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>116</XPosition>
                        <YPosition>452</YPosition>
                        <TabLabel>corporation_address2</TabLabel>
                        <TabName>Corporation Address2</TabName>
                        <TabValue>adsfasdf</TabValue>
                        <DocumentID>47977</DocumentID>
                        <PageNumber>3</PageNumber>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>Custom</TabType>
                        <Status>Signed</Status>
                        <XPosition>116</XPosition>
                        <YPosition>512</YPosition>
                        <TabLabel>corporation_address3</TabLabel>
                        <TabName>Corporation Address3</TabName>
                        <TabValue>asdfadf</TabValue>
                        <DocumentID>47977</DocumentID>
                        <PageNumber>3</PageNumber>
                        <CustomTabType>Text</CustomTabType>
                    </TabStatus>
                    <TabStatus>
                        <TabType>DateSigned</TabType>
                        <Status>Signed</Status>
                        <XPosition>735</XPosition>
                        <YPosition>110</YPosition>
                        <TabLabel>date</TabLabel>
                        <TabName>Date</TabName>
                        <TabValue>12/17/2020</TabValue>
                        <DocumentID>47977</DocumentID>
                        <PageNumber>3</PageNumber>
                    </TabStatus>
                </TabStatuses>
                <RecipientAttachment>
                    <Attachment>
                        <Data>redacted by example as it looks to be a base64 encoded string</Data>
                        <Label>DSXForm</Label>
                    </Attachment>
                </RecipientAttachment>
                <AccountStatus>Active</AccountStatus>
                <EsignAgreementInformation>
                    <AccountEsignId>f78e337a-a9c7-47e6-bc20-6f75a84706ba</AccountEsignId>
                    <UserEsignId>d8b49626-cf1d-41dc-bc3f-9478a57036ff</UserEsignId>
                    <AgreementDate>2020-12-17T07:44:08.503</AgreementDate>
                </EsignAgreementInformation>
                <FormData>
                    <xfdf>
                        <fields>
                            <field name="signatory_name">
                                <value>Example Signatory</value>
                            </field>
                            <field name="signatory_email">
                                <value>example@example.org</value>
                            </field>
                            <field name="corporation_name">
                                <value>The Linux Foundation</value>
                            </field>
                            <field name="cla_manager_name">
                                <value>Example Signatory</value>
                            </field>
                            <field name="cla_manager_email">
                                <value>example@example.org</value>
                            </field>
                            <field name="signatory_title">
                                <value>CEO</value>
                            </field>
                            <field name="corporation_address1">
                                <value>113</value>
                            </field>
                            <field name="corporation_address2">
                                <value>adsfasdf</value>
                            </field>
                            <field name="corporation_address3">
                                <value>asdfadf</value>
                            </field>
                            <field name="DateSigned">
                                <value>12/17/2020</value>
                            </field>
                        </fields>
                    </xfdf>
                </FormData>
                <RecipientId>dac1279d-7cc7-4a34-84ae-be0bff04af9b</RecipientId>
            </RecipientStatus>
        </RecipientStatuses>
        <TimeGenerated>2020-12-17T07:44:53.0177631</TimeGenerated>
        <EnvelopeID>c915984a-f761-4c28-ac2c-767253ba3362</EnvelopeID>
        <Subject>EasyCLA: CLA Signature Request for CommonTraceFormat</Subject>
        <UserName>Example Signatory</UserName>
        <Email>example@example.org</Email>
        <Status>Completed</Status>
        <Created>2020-12-17T07:43:55.687</Created>
        <Sent>2020-12-17T07:43:56.233</Sent>
        <Delivered>2020-12-17T07:44:08.707</Delivered>
        <Signed>2020-12-17T07:44:30.673</Signed>
        <Completed>2020-12-17T07:44:30.673</Completed>
        <ACStatus>Original</ACStatus>
        <ACStatusDate>2020-12-17T07:43:55.687</ACStatusDate>
        <ACHolder>Example Signatory</ACHolder>
        <ACHolderEmail>example@example.org</ACHolderEmail>
        <ACHolderLocation>DocuSign</ACHolderLocation>
        <SigningLocation>Online</SigningLocation>
        <SenderIPAddress>3.237.106.64</SenderIPAddress>
        <EnvelopePDFHash/>
        <CustomFields>
            <CustomField>
                <Name>AccountId</Name>
                <Show>false</Show>
                <Required>false</Required>
                <Value>10406522</Value>
                <CustomFieldType>Text</CustomFieldType>
            </CustomField>
            <CustomField>
                <Name>AccountName</Name>
                <Show>false</Show>
                <Required>false</Required>
                <Value>Linux Foundation</Value>
                <CustomFieldType>Text</CustomFieldType>
            </CustomField>
            <CustomField>
                <Name>AccountSite</Name>
                <Show>false</Show>
                <Required>false</Required>
                <Value>demo</Value>
                <CustomFieldType>Text</CustomFieldType>
            </CustomField>
        </CustomFields>
        <AutoNavigation>true</AutoNavigation>
        <EnvelopeIdStamping>true</EnvelopeIdStamping>
        <AuthoritativeCopy>false</AuthoritativeCopy>
        <DocumentStatuses>
            <DocumentStatus>
                <ID>47977</ID>
                <Name>Apache Style</Name>
                <TemplateName/>
                <Sequence>1</Sequence>
            </DocumentStatus>
        </DocumentStatuses>
    </EnvelopeStatus>
    <TimeZone>Pacific Standard Time</TimeZone>
    <TimeZoneOffset>-8</TimeZoneOffset>
</DocuSignEnvelopeInformation> 
    """
    tree = ET.fromstring(content)

    signature = Signature()
    populate_signature_from_ccla_callback(content, tree, signature)
    assert signature.get_user_docusign_name() == "Example Signatory"
    assert signature.get_user_docusign_date_signed() == "2020-12-17T07:44:08.503"
    assert signature.get_user_docusign_raw_xml() == content
    assert signature.get_signing_entity_name() == "The Linux Foundation"
    assert "user_docusign_name" in signature.to_dict()
    assert "signing_entity_name" in signature.to_dict()
    assert "user_docusign_date_signed" in signature.to_dict()
    assert "user_docusign_raw_xml" not in signature.to_dict()
    assert "user_docusign_name" in str(signature)
    assert "user_docusign_date_signed" in str(signature)
    assert "user_docusign_raw_xml" not in str(signature)


def test_populate_signature_from_icla_callback():
    tree = ET.fromstring(content_icla_agreement_date)

    agreement_date = "2020-12-21T08:29:20.51"

    signature = Signature()
    populate_signature_from_icla_callback(content_icla_agreement_date, tree, signature)
    assert signature.get_user_docusign_name() == "Example FullName"
    assert signature.get_user_docusign_date_signed() == agreement_date
    assert signature.get_user_docusign_raw_xml() == content_icla_agreement_date
    assert "user_docusign_name" in signature.to_dict(), ""
    assert "user_docusign_date_signed" in signature.to_dict()
    assert "user_docusign_raw_xml" not in signature.to_dict()
    assert "user_docusign_name" in str(signature)
    assert "user_docusign_date_signed" in str(signature)
    assert "user_docusign_raw_xml" not in str(signature)


def test_populate_signature_missing_agreement_date():
    tree = ET.fromstring(content_icla_missing_agreement_date)

    signed_date = "2020-12-21T08:30:10.133"
    signature = Signature()
    populate_signature_from_icla_callback(content_icla_agreement_date, tree, signature)
    assert signature.get_user_docusign_name() == "Example FullName"
    assert signature.get_user_docusign_date_signed() == signed_date
    assert signature.get_user_docusign_raw_xml() == content_icla_agreement_date


def test_create_default_company_values():
    company = Company(
        company_name="Google",
    )

    values = create_default_company_values(
        company=company,
        signatory_name="Signatory1",
        signatory_email="signatory@example.com",
        manager_name="Manager1",
        manager_email="manager@example.com",
        schedule_a="Schedule"
    )

    assert "corporation_name" in values
    assert "corporation" in values

    company = Company(
        company_name="Google",
        signing_entity_name="Google1"
    )

    values = create_default_company_values(
        company=company,
        signatory_name="Signatory1",
        signatory_email="signatory@example.com",
        manager_name="Manager1",
        manager_email="manager@example.com",
        schedule_a="Schedule"
    )

    assert "corporation_name" in values
    assert "corporation" in values

    values = create_default_company_values(
        company=None,
        signatory_name="Signatory1",
        signatory_email="signatory@example.com",
        manager_name="Manager1",
        manager_email="manager@example.com",
        schedule_a="Schedule"
    )

    assert "corporation_name" not in values
    assert "corporation" not in values


def test_document_signed_email_content():
    user = User()
    user.set_user_id("user_id_value")
    user.set_user_name("john")

    p = Project(
        project_id="project_id_value",
        project_name="JohnsProject",
    )

    s = Signature(
        signature_reference_id="signature_reference_id_value"
    )

    subject, body = document_signed_email_content(
        icla=False,
        project=p,
        signature=s,
        user=user
    )

    assert subject is not None
    assert body is not None

    assert "Signed for JohnsProject" in subject
    assert "Hello john" in body
    assert "EasyCLA regarding the project JohnsProject" in body
    assert "The CLA has now been signed." in body
    assert "alt=\"CCLA Document Link\"" in body

    # try with different recipient names
    user.set_user_name(None)
    user.set_lf_username("johnlf")

    subject, body = document_signed_email_content(
        icla=False,
        project=p,
        signature=s,
        user=user
    )

    assert "Hello johnlf" in body

    user.set_lf_username(None)

    subject, body = document_signed_email_content(
        icla=False,
        project=p,
        signature=s,
        user=user
    )

    assert "Hello CLA Manager" in body

    subject, body = document_signed_email_content(
        icla=True,
        project=p,
        signature=s,
        user=user
    )

    assert "Signed for JohnsProject" in subject
    assert "Hello Contributor" in body
    assert "EasyCLA regarding the project JohnsProject" in body
    assert "The CLA has now been signed." in body
    assert "alt=\"ICLA Document Link\"" in body
    assert "EasyCLA CLA Manager console" not in body


def test_cla_signatory_email_content():
    params = ClaSignatoryEmailParams(
        cla_group_name="cla_group_name_value",
        signatory_name="signatory_name_value",
        cla_manager_name="john",
        cla_manager_email="john@example.com",
        company_name="IBM",
        project_version="v1",
        project_names=["project1", "project2"]
    )

    email_subject, email_body = cla_signatory_email_content(params)
    assert "EasyCLA: CLA Signature Request for cla_group_name_value" == email_subject
    assert "<p>Hello signatory_name_value,<p>" in email_body
    assert "EasyCLA regarding the project(s) project1, project2 associated" in email_body
    assert "with the CLA Group cla_group_name_value" in email_body
    assert "john has designated you as an authorized signatory" in email_body
    assert "signatory for the organization IBM" in email_body
    assert "<p>After you sign, john (as the initial CLA Manager for your company)" in email_body
    assert "and if you approve john as your initial CLA Manager" in email_body
    assert "contact the requester at john@example.com" in email_body
