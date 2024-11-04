import unittest
from unittest.mock import Mock, patch
import datetime

from cla.models.docusign_models import DocuSign

def test_save_employee_signature(project, company, user_instance):
    """ Test _save_employee_signature """
    # Mock DocuSign method and related class methods
    DocuSign.check_and_prepare_employee_signature = Mock(return_value={'success': {'the employee is ready to sign the CCLA'}})
    
    # Create an instance of DocuSign and mock its dynamo_client
    docusign = DocuSign()
    docusign.dynamo_client = Mock()  # Mock the dynamo_client on the instance
    mock_put_item = docusign.dynamo_client.put_item = Mock()

    # Mock ecla signature object with necessary attributes for the helper method
    signature = Mock()
    signature.get_signature_id.return_value = "sig_id"
    signature.get_signature_project_id.return_value = "proj_id"
    signature.get_signature_document_minor_version.return_value = 1
    signature.get_signature_document_major_version.return_value = 2
    signature.get_signature_reference_id.return_value = "ref_id"
    signature.get_signature_reference_type.return_value = "user"
    signature.get_signature_type.return_value = "cla"
    signature.get_signature_signed.return_value = True
    signature.get_signature_approved.return_value = True
    signature.get_signature_acl.return_value = ['acl1', 'acl2']
    signature.get_signature_user_ccla_company_id.return_value = "company_id"
    signature.get_signature_return_url.return_value = None
    signature.get_signature_reference_name.return_value = None

    # Call the helper method
    docusign._save_employee_signature(signature)

    # Check if dynamo_client.put_item was called
    assert mock_put_item.called

    # Extract the 'Item' argument passed to put_item
    _, kwargs = mock_put_item.call_args
    item = kwargs['Item']

    # Assert that 'date_modified' and 'date_created' are in the item
    assert 'date_modified' in item
    assert 'date_created' in item


    # Optionally, check if they are correctly formatted ISO timestamps
    try:
        datetime.datetime.fromisoformat(item['date_modified']['S'])
        datetime.datetime.fromisoformat(item['date_created']['S'])
    except ValueError:
        assert False, "date_modified or date_created are not valid ISO format timestamps"
