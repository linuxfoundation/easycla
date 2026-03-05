# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""Unit tests validating the PyJWT migration does not break JWT/auth processing.

These tests are intentionally offline:
- The Auth0 JWKS fetch in `cla.auth` is mocked.
- RSA keys are generated on the fly.

Run:
  cd cla-backend
  python -m unittest cla.tests.unit.test_jwt_auth
"""

import base64
import importlib
import os
import time
import unittest
from types import SimpleNamespace
from unittest.mock import patch

import jwt
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa


def _b64url_uint(val: int) -> str:
    """Base64url encode an integer without padding (RFC7517-compatible)."""
    raw = val.to_bytes((val.bit_length() + 7) // 8, "big")
    return base64.urlsafe_b64encode(raw).decode("ascii").rstrip("=")


def _generate_rsa_jwks(kid: str = "test-kid"):
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    public_numbers = key.public_key().public_numbers()

    jwk = {
        "kty": "RSA",
        "kid": kid,
        "use": "sig",
        "n": _b64url_uint(public_numbers.n),
        "e": _b64url_uint(public_numbers.e),
    }
    jwks = {"keys": [jwk]}

    private_pem = key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    )
    public_pem = key.public_key().public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    )
    return private_pem, public_pem, jwks


class _MockResponse:  # pylint: disable=too-few-public-methods
    def __init__(self, jwks, status_code: int = 200):
        self._jwks = jwks
        self.status_code = status_code

    def json(self):
        return self._jwks

    def raise_for_status(self):  # pragma: no cover
        return None


class TestPyJwtMigration(unittest.TestCase):
    def setUp(self):
        # Preserve environment; cla.auth reads env vars at import time.
        self._orig_env = os.environ.copy()

        os.environ["AUTH0_DOMAIN"] = "example.invalid"
        os.environ["AUTH0_USERNAME_CLAIM"] = "nickname"
        os.environ["AUTH0_EMAIL_CLAIM"] = "email"
        os.environ["AUTH0_ALGORITHM"] = "RS256"

        import cla.auth  # pylint: disable=import-outside-toplevel

        # Reload to pick up env vars configured above.
        self.auth = importlib.reload(cla.auth)

    def tearDown(self):
        os.environ.clear()
        os.environ.update(self._orig_env)

    def test_authenticate_user_valid_rs256(self):
        private_pem, _public_pem, jwks = _generate_rsa_jwks()
        now = int(time.time())

        token = jwt.encode(
            {
                "sub": "user|123",
                "nickname": "nick",
                "email": "a@example.com",
                "iat": now,
                "exp": now + 3600,
            },
            private_pem,
            algorithm="RS256",
            headers={"kid": "test-kid"},
        )

        with patch.object(self.auth.requests, "get", return_value=_MockResponse(jwks)):
            user = self.auth.authenticate_user({"Authorization": f"Bearer {token}"})

        self.assertEqual(user.sub, "user|123")
        self.assertEqual(user.username, "nick")
        self.assertEqual(user.email, "a@example.com")

    def test_authenticate_user_expired_token(self):
        private_pem, _public_pem, jwks = _generate_rsa_jwks()
        now = int(time.time())

        token = jwt.encode(
            {
                "sub": "user|123",
                "nickname": "nick",
                "email": "a@example.com",
                "iat": now - 60,
                "exp": now - 1,
            },
            private_pem,
            algorithm="RS256",
            headers={"kid": "test-kid"},
        )

        with patch.object(self.auth.requests, "get", return_value=_MockResponse(jwks)):
            with self.assertRaises(self.auth.AuthError) as ctx:
                self.auth.authenticate_user({"Authorization": f"Bearer {token}"})

        self.assertEqual(ctx.exception.response, "token is expired")

    def test_authenticate_user_invalid_signature(self):
        # JWKS uses key A, token signed with key B but same kid.
        _priv_a, _pub_a, jwks = _generate_rsa_jwks(kid="test-kid")
        priv_b, _pub_b, _jwks_b = _generate_rsa_jwks(kid="test-kid")
        now = int(time.time())

        token = jwt.encode(
            {
                "sub": "user|123",
                "nickname": "nick",
                "email": "a@example.com",
                "iat": now,
                "exp": now + 3600,
            },
            priv_b,
            algorithm="RS256",
            headers={"kid": "test-kid"},
        )

        with patch.object(self.auth.requests, "get", return_value=_MockResponse(jwks)):
            with self.assertRaises(self.auth.AuthError):
                self.auth.authenticate_user({"Authorization": f"Bearer {token}"})

    def test_cla_user_unverified_claims(self):
        # cla.user depends on hug; skip if not installed in the test env.
        try:
            import hug  # noqa: F401  pylint: disable=unused-import,import-outside-toplevel
        except ModuleNotFoundError:
            self.skipTest("hug is not installed")

        import cla.user  # pylint: disable=import-outside-toplevel

        importlib.reload(cla.user)

        token = jwt.encode(
            {
                "sub": "user|456",
                "preferred_username": "nick2",
                "email": "b@example.com",
            },
            "secret",
            algorithm="HS256",
        )

        req = SimpleNamespace(headers={"Authorization": f"Bearer {token}"})
        user = cla.user.cla_user(request=req)

        self.assertIsNotNone(user)
        self.assertEqual(user.user_id, "user|456")
        self.assertEqual(user.preferred_username, "nick2")
        self.assertEqual(user.email, "b@example.com")
