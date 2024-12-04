EasyCLA Sign Flow: Sequence Overview

1. *User Creates a Pull Request (PR)*
    ◦ A contributor initiates a PR in the repository hosted on GitHub, Gerrit, or GitLab.
2. *Repository Triggers Activity Endpoint*
• The repository platform sends a request to EasyCLA’s Python endpoint:
    ◦ v2/repository-provider/{provider}/activity
3. *EasyCLA Checks User Authorization*
    ◦ EasyCLA internally verifies if the users involved in the PR are authorized to contribute to the repository.
4. *Update Repository with User Status*
    ◦ EasyCLA communicates back to the repository provider, updating the status of each user as either *signed* or *not signed*.
5. *User Initiates Sign Process*
    ◦ If a user is marked as *not signed*, they are prompted to begin the signing process and are redirected to the EasyCLA Contributor Console.
6. *Contributor Chooses Sign Type*
• Upon reaching the Contributor Console, the user selects one of two options:
        ▪︎ *Individual Contributor*
    ◦ *Corporate Contributor*
7. *Individual Contributor Flow*
• *a. Initiate Individual Signature Request*
• The system invokes the Go-based endpoint:
        ▪︎ v4/request-individual-signature
    ◦ This action creates a new signature record with `signed = false` and initiates the signing process.
• *a1. Redirect to DocuSign*
    ◦ The API handles the integration with DocuSign, preparing a callback and redirect URL, and redirects the user to DocuSign for signing.
• *a2. Completion of Signing*
• Once the user completes the signing on DocuSign, a callback is triggered to:
        ▪︎ v4/signed/individual/{installation_id}/{github_repository_id}/{change_request_id}
    ◦ This endpoint updates the signature record’s `signed` flag to `true`, completing the process.
8. *Corporate Contributor Flow*
• *b. Initiate Corporate Signature Process*
9. *Redirect to Company Search*
        ▪︎ The user is redirected to a company search interface within the Contributor Console.
10. *Search for Company*
• Upon selecting a company, the system calls the Go-based search endpoint:
            • v3/organization/search?companyName=Info&amp;include-signing-entity-name=false
        ▪︎ This retrieves the relevant company information.
11. *Check and Prepare Employee Signature*
• The system invokes the Python endpoint:
            • v2/check-prepare-employee-signature
            • This checks whether the company follows a Corporate CLA (CCLA) or an Entity CLA (ECLA) flow.
• *i. If Company Has a CCLA:*
                ◦ The system verifies if the user is authorized.
                ◦ If *not authorized*, it prompts the user to contact the existing CLA manager for authorization.
• The Go-based endpoint sends a notification to CLA managers:
                ◦ v4/notify-cla-managers
            • An email is sent to the CLA managers, and the process ends.
• *ii. If Company Does Not Have a CCLA:*
                ◦ The system checks if the user is a CLA manager.
• *A. User is a CLA Manager:*
• Assigns CLA manager designee permissions via:
                ◦ v4/company/{companySFID}/user/{userLFID}/claGroupID/{claGroupID}/cla-manager-designee
• Verifies the assigned role:
                ◦ v4/company/{companySFID}/user/{userLFID}/claGroupID/{claGroupID}/is-cla-manager-designee
• If the role is confirmed, it calls the endpoint to request a corporate signature:
                ◦ v4/request-corporate-signature
                ◦ This creates the signature record, completing the process.
• *B. User is Not a CLA Manager:*
• Fetches company administrators using:
                ◦ v4/company/{companySFID}/admin
• Sends an invitation to become a company admin via:
                ◦ /user/{userID}/invite-company-admin
    ◦ An email is sent to the user to invite them as a company admin, concluding the process.
