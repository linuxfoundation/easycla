# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
import json
import unittest
import uuid
from unittest.mock import Mock

import cla
from cla.controllers.signature import notify_whitelist_change
from cla.controllers.signing import canceled_signature_html
from cla.models.dynamo_models import Project, Signature, User
from cla.models.sns_email_models import MockSNS
from cla.user import CLAUser


def test_canceled_signature_html():
    signature_type = "ccla"
    signature_return_url = "https://github.com/communitybridge/easycla/pull/227"
    signature_sign_url = "https://demo.docusign.net/Signing/MTRedeem/v1/4b594c99-d76b-46c4-bf8c-5912b177b0eb?slt=eyJ0eXAiOi"
    signature = Signature(
        signature_type=signature_type,
        signature_return_url=signature_return_url,
        signature_sign_url=signature_sign_url
    )

    result = canceled_signature_html(signature=signature)
    assert "Ccla" in result
    assert signature_return_url in result
    assert signature_sign_url in result

    signature = Signature(
        signature_sign_url=signature_sign_url
    )
    result = canceled_signature_html(signature=signature)

    assert "Ccla" not in result
    assert signature_return_url not in result
    assert signature_sign_url in result


class TestSignatureController(unittest.TestCase):
    def test_notify_whitelist_change(self):
        old_sig = Signature()
        new_sig = Signature()
        new_sig.set_signature_reference_name('Company')
        new_sig.set_signature_project_id('projectID')
        cla_manager = CLAUser({'name': 'CLA Manager'})
        old_sig.set_domain_whitelist(['a.com', 'b.com'])
        new_sig.set_domain_whitelist(['b.com', 'd.com'])

        old_sig.set_github_whitelist([])
        new_sig.set_github_whitelist(['githubuser'])

        old_sig.set_email_whitelist(['whitelist.email@gmail.com'])
        new_sig.set_email_whitelist([])

        old_sig.set_github_org_whitelist(['githuborg'])
        new_sig.set_github_org_whitelist(['githuborg'])

        snsClient = MockSNS()
        cla.controllers.signature.get_email_service = Mock()
        cla.controllers.signature.get_email_service.return_value = snsClient
        new_sig.get_managers = Mock(side_effect=mock_get_managers)

        cla.models.dynamo_models.Project.load = Mock(side_effect=mock_project)
        cla.models.dynamo_models.Project.get_project_name = Mock()
        cla.models.dynamo_models.Project.get_project_name.return_value = 'Project'
        cla.models.dynamo_models.User.get_user_by_github_username = Mock(side_effect=mock_get_user_by_github_username)
        notify_whitelist_change(cla_manager, old_sig, new_sig)
        self.assertEqual(len(snsClient.emails_sent), 3)
        # check email to cla manager
        msg = snsClient.emails_sent[0]
        msg = json.loads(msg)
        self.assertEqual(msg['data']['subject'], 'EasyCLA: Approval List Update for Project')
        self.assertEqual(msg['data']['recipients'], ['cla_manager1@gmail.com', 'cla_manager2@gmail.com'])
        body = msg['data']['body']
        self.assertIn('a.com', body)
        self.assertNotIn('b.com', body)
        self.assertIn('d.com', body)
        self.assertIn('githubuser', body)
        self.assertIn('whitelist.email@gmail.com', body)
        self.assertNotIn('githuborg', body)
        # check email sent to contributor - removed email
        msg = snsClient.emails_sent[1]
        msg = json.loads(msg)
        self.assertEqual(msg['data']['subject'], 'EasyCLA: Approval List Update for Project')
        self.assertEqual(msg['data']['recipients'], ['whitelist.email@gmail.com'])
        body = msg['data']['body']
        self.assertIn('deleted', body)
        self.assertIn('Company', body)
        self.assertIn('Project', body)
        self.assertIn('CLA Manager', body)
        # check email sent to contributor - added github user
        msg = snsClient.emails_sent[2]
        msg = json.loads(msg)
        self.assertEqual(msg['data']['subject'], 'EasyCLA: Approval List Update for Project')
        self.assertEqual(msg['data']['recipients'], ['user1@gmail.com'])
        body = msg['data']['body']
        self.assertIn('added', body)
        self.assertIn('Company', body)
        self.assertIn('Project', body)
        self.assertIn('CLA Manager', body)


def mock_get_managers():
    u1 = User()
    u1.set_lf_email('cla_manager1@gmail.com')
    u2 = User()
    u2.set_lf_email('cla_manager2@gmail.com')
    return [u1, u2]


def mock_project(project_id):
    self = Project()
    return self


def mock_get_user_by_github_username(username):
    u1 = User()
    u1.set_user_email('user1@gmail.com')
    return [u1]

def test_signature_acl():
    sig = Signature()
    sig.set_signature_document_major_version(1)
    sig.set_signature_document_minor_version(0)
    sig.set_signature_id(str(uuid.uuid4()))
    sig.set_signature_project_id(str(uuid.uuid4()))
    sig.set_signature_reference_id(str(uuid.uuid4()))
    sig.set_signature_type('user')
    sig.set_signature_acl('lgryglicki')
    # print(f"signature_id1 {sig.get_signature_id()}")
    # sig.save()
    # sig2 = Signature()
    # sig2.load(signature_id='afcf787b-8010-4c43-8bf7-2dbbfa229f2c')
    # print(f"signature_id2 {sig2.get_signature_id()}")
    # sig2.set_signature_id(str(uuid.uuid4()))
    # print(f"signature_id3 {sig2.get_signature_id()}")
    # sig2.save()

if __name__ == '__main__':
    unittest.main()
