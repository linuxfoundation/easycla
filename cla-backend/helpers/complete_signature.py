# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Convenience script to complete a signature (manually fire the DocuSign callback).
This helper script is useful if you can't expose your CLA system to the internet for testing
with DocuSign directly.
"""
import sys
sys.path.append('../')

if len(sys.argv) != 3:
    print('\nUsage: python3 %s <signature-id> <docusign-envelope-id>\n' %sys.argv[0])
    print('This helper script should be used after the POST to /v1/request-signature endpoint')
    print('You can find the signature ID in the response body, or by using the /v1/signature endpoint')
    print('You can find the envelope ID through the logs when creating the signature object, or through the DocuSign web UI if you have access')
    print('This script will need to be run from inside the CLA container\n')
    print('Note that the updated PR will not contain working links to sign if you are using this script because your CLA instance is not web-accessible')
    exit()

SIGNATURE_ID = sys.argv[1]
ENVELOPE_ID = sys.argv[2]
INSTALLATION_ID = 49309 # Assumed to be testing on the CLA-Test repository

import cla
from cla.utils import get_signature_instance, get_user_instance, get_signing_service, \
                      get_active_signature_metadata, delete_active_signature_metadata
from cla.models.docusign_models import update_repository_provider

cla.log.info('Completing the signature: %s' %SIGNATURE_ID)
signature = get_signature_instance()
signature.load(SIGNATURE_ID)
signature.set_signature_signed(True)
signature.set_signature_embargo_acked(True)
signature.save()
if signature.get_signature_reference_type() != 'user':
    cla.log.error('Trying to handle CCLA as a ICLA - not implemented yet')
    raise NotImplementedError()
user = get_user_instance()
user.load(signature.get_signature_reference_id())
# Remove the active signature metadata.
metadata = get_active_signature_metadata(user.get_user_id())
delete_active_signature_metadata(user.get_user_id())
# Send email with signed document.
cla.log.info('Sending signed document to user - see Mailhog')
docusign = get_signing_service()
docusign.send_signed_document(ENVELOPE_ID, user)
# Update the repository provider with this change.
cla.log.info('Updating GitHub PR...')
update_repository_provider(INSTALLATION_ID, metadata['repository_id'], metadata['pull_request_id'])
cla.log.info('Done')
