# DynamoDB Tables

Copyright The Linux Foundation and each contributor to CommunityBridge.

SPDX-License-Identifier: CC-BY-4.0

## Companies

| Column Name | Description |
| :--- | :--- |
| company\_id | Primary Key UUID for Companies table |
| company\_acl | List of CLA Managers of the Company |
| company\_name | Name of the Company |
| company\_manager\_id | Initial Company Manager who created the company |

## Company-Invites

| Column Name | Description |
| :--- | :--- |
| company\_invite\_id | Primary Key UUID for Company Invites table |
| requested\_company\_id | Company that the pending invitation will be sent to\* |
| user\_id | user requesting invitation |

* When a user tries to create a company that already exists in the Corporate Console, the user can send a request to the CLA Managers to be included in the Company ACL. A row in this table represents a pending request that any CLA Manager of the company can accept or decline in the Corporate Console.

## Gerrit-Instances

| Column Name | Description |
| :--- | :--- |
| gerrit\_id | Primary Key UUID for Gerrit Instances table |
| gerrit\_name | Name of the Gerrit Instance |
| gerrit\_url | the URL to the Gerrit Instance |
| group\_id\_icla | The LDAP Group ID for ICLA for this Gerrit Instance\* |
| group\_name\_icla | The LDAP Group Name for ICLA |
| group\_id\_ccla | The LDAP ID for CCLA regarding this Gerrit Instance |
| group\_name\_ccla | The LDAP Group Name for CCLA |
| project\_id | The CLA Group ID that the Gerrit Instance is configured with |

* When you sign an ICLA or a CCLA for this Gerrit Instance, you will be added to this LDAP Group. Once you are part of this group, you will be able to make a contribution to the repositories of the Gerrit Instance.

## Github-Orgs

| Column Name | Description |
| :--- | :--- |
| organization\_name | Primary Key String Identifier for Github Orgs Table. |
| organization\_installation\_id | The ID assigned by Github when the CLA App has been installed to the Github Organization |
| organization\_sfid | The SFDC ID that the Github Organization is configured to. |

* Github Organizations can be configured on the Project Management Console. They will be tied to individual SFDC IDs.

## Repositories

| Column Name | Description |
| :--- | :--- |
| repository\_id | Primary Key UUID for the Repositories table \(Github repositories that have been configured under a CLA Group\) |
| repository\_external\_id | The name of the CLA Group |
| repository\_name | Name of the repository configured |
| repository\_organization\_name | Name of the Github org that includes the repository |
| repository\_project\_id | The CLA Group ID that has the Github Repository configured |
| repository\_sfdc\_id | The SFDC ID that the repository is configured under |
| repository\_type | The repository provider \(currently only will have github\) |
| repository\_url | History of ICLA Templates that the project manager created.\* |

## Projects

| Column Name | Description |
| :--- | :--- |
| project\_id | Primary Key UUID for the Projects table \(Referred to as CLA Groups on Project Management Console\) |
| project\_name | The name of the CLA Group |
| project\_acl | The Cla Group Manager ACL \(Access Control List\) |
| project\_ccla\_enabled | The boolean flag that determines whether the CLA Group will include a CCLA. |
| project\_icla\_enabled | The boolean flag that determines whether the CLA Group will include an ICLA. |
| project\_ccla\_requires\_icla\_signature | The boolean flag that determines when an employee has signed a CCLA, the employee needs to sign an ICLA as well to pass the CLA checks |
| project\_corporate\_documents | History of CCLA Templates that the project manager created.\* |
| project\_corporate\_documents | History of ICLA Templates that the project manager created.\* |

### The Templates will be saved as a list of Documents, that have the following properties

| Column Name | Description |
| :--- | :--- |
| document\_author\_name | Name of the author of the document \(deprecated\) |
| document\_creation\_date | Date and time when the document was created through the Project Management Console |
| document\_content\_type | Document file type. Currently only supports "storage+pdf" |
| document\_file\_id | UUID unique to the document generated for management under S3 and Docusign API |
| document\_legal\_entity\_name | The legal entity name for the document \(deprecated\) |
| document\_major\_version | The major version of the Document |
| document\_minor\_version | The minor version of the Document |
| document\_name | Name of the CLA Template |
| document\_preamble | Preamble of the CLA Template \(deprecated\) |
| document\_s3\_url | The URL to the generated CLA Template on the AWS S3 Bucket |
| document\_tabs | Document custom tabs that a Signer must fill before completing the singing process, such as Corporate Name, E-mail, Address, etc. \* |

### Each Document will have a list of document\_tabs, that have the following properties

| Column Name | Description |
| :--- | :--- |
| document\_tab\_anchor\_string | The anchor string \(Docusign looks for this string in the document to create the custom tab\) |
| document\_tab\_anchor\_ignore\_if\_present | the flag to ignore the custom tab when Docusign is not able to locate the anchor string |
| document\_tab\_anchor\_x\_offset | The anchor string X offsets for the custom tab \(in pixels\) |
| document\_tab\_anchor\_y\_offset | The anchor string Y offsets for the custom tab \(in pixels\) |
| document\_tab\_id | The custom tab ID identifier |
| document\_tab\_is\_locked | Flag to determine if the custom tab is editable |
| document\_tab\_is\_required | Flag to determine if the custom tab is optional |
| document\_tab\_name | custom tab name that displays when you hover the mouse on the tab |
| document\_tab\_type | type of custom tab \(e.g. "text", "sign", "date"\) |
| document\_tab\_position\_x | the X coordinates that the custom tab will be located \(deprecated due to anchor string\) |
| document\_tab\_position\_y | the Y coordinates that the custom tab will be located \(deprecated due to anchor string\) |
| document\_tab\_page | The page number of the custom tab |
| document\_tab\_width | width of the custom tab |
| document\_tab\_height | height of the custom tab |

## Signature

| Column Name | Description |
| :--- | :--- |
| signature\_id | Primary Key UUID for the Signatures table |
| signature\_acl | The Signature Access Control List |
| signature\_approved | Flag that determines if signature is approved |
| signature\_signed | Flag that determines if signature has been signed |
| signature\_document\_major\_version | The major version number for the signed document |
| signature\_document\_minor\_version | The minor version number for the signed document |
| signature\_project\_id | The CLA Group ID that the signature refers to |
| signature\_reference\_id | The ID of the owner of the signature \(user\_id or company\_id\) |
| signature\_reference\_type | The type of the owner \(user/company\) |
| signature\_user\_ccla\_company\_id | The company ID that the user is signing the employee CCLA for |
| signature\_return\_url | The return url that the user will be directed to when the user has completed the signing process \(The Github Pull Request Page, the Gerrit Instance page\) |
| signature\_type | Determines whether the signature is for an ICLA \(cla\) or a CCLA \(ccla\) |
| signature\_callback\_url | When a user completes the signing process, Docusign sends the CLA System a webhook that contains information about the document \(i.e. user finished signing, user opened document\). The URL that the Docusign calls on the CLA System  so that the system can mark the signature as signed |
| signature\_envelope\_id | the envelope\_id generated by Docusign \(used for voiding signatures that are no longer valid\) |
| signature\_sign\_url | The Docusign URL that the user will be directed to fill the custom tabs and sign the document |
| domain\_whitelist | The domain approved list for the CCLA signature. Accepts wildcards on URLs |
| email\_whitelist | The email approved list for the CCLA signature |
| github\_whitelist | The Github username approved list for the CCLA signature |
| github\_org\_whitelist | The Github organization approved list for the CCLA signature |

## User-Permissions

| Column Name | Description |
| :--- | :--- |
| username | Primary Key Username for the User permissions |
| projects | List of SFDC projects that the user has access to in the Project Management Console |

## Users

| Column Name | Description |
| :--- | :--- |
| user\_id | Primary Key UUID for the Users Table \(users within the CLA system\) |
| user\_name | The Full name of the user |
| lf\_email | The LF Email for the user |
| lf\_username | The LFID of the user |
| user\_emails | The verified list of emails for a Github User |
| user\_github\_id | The github ID \(i.e. 23942335\) for a GIthub user |
| user\_github\_username | The github username for user |
| user\_company\_id | The company ID that the user will have when the user completes an employee CCLA |

## Session-Store

| Column Name | Description |
| :--- | :--- |
| id | Primary Key ID for the Session Stores table \(Session stored for Github Oauth\) |
| options | The options for the session \(Domain, HttpOnly, MaxAge, Path, SameSite, Secure\) flags |
| values | The hashed session token for this session |

## Store

| Column Name | Description |
| :--- | :--- |
| key | Primary Key UUID for the Store table \(Temporary Key-value storage for user information during the signing process\) |
| expire \(TTL\) | Expiration date time for the key-value pair in epoch |
| value | Temporary values stored for the key-value |

