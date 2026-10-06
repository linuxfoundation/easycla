# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


MOCK_CURL = r"""#!/usr/bin/env python3
import base64
import hashlib
import json
import os
from pathlib import Path
import sys
import time
from urllib.parse import parse_qs, urlsplit

args = sys.argv[1:]
root = Path(os.environ["MOCK_ROOT"])
state_file = root / "state.json"
state = json.loads(state_file.read_text()) if state_file.exists() else {}
url = next(arg for arg in args if arg.startswith(("https://", "http://")))
parts = urlsplit(url)
body = sys.stdin.read() if "--data" in args else ""
with (root / "requests.jsonl").open("a") as output:
    output.write(json.dumps({"args": args, "url": url, "body": body}) + "\n")

if parts.path == "/authorize":
    query = parse_qs(parts.query)
    state.update({key: values[0] for key, values in query.items()})
    state["issuer"] = parts.scheme + "://" + parts.netloc + "/"
    print(state["issuer"] + "login?state=interaction", end="")
elif parts.path == "/login":
    Path(args[args.index("-c") + 1]).write_text(
        parts.hostname + "\tFALSE\t/\tTRUE\t0\t_csrf\tcsrf\n"
    )
elif parts.path == "/usernamepassword/login":
    state["login"] = json.loads(body)
    if os.environ.get("MOCK_LOGIN_FAILURE"):
        print(json.dumps({"error": "access_denied"}), end="")
    else:
        print('<input name="wa" value="signin"><input name="wresult" value="result&amp;value">'
              '<input name="wctx" value="context">', end="")
elif parts.path == "/login/callback":
    assert parse_qs(body)["wresult"] == ["result&value"]
    print(state["issuer"] + "authorize/resume?state=resume", end="")
elif parts.path == "/authorize/resume":
    print(state["redirect_uri"] + "?code=fixture-code", end="")
elif parts.path == "/oauth/token":
    request = json.loads(body)
    assert request["grant_type"] == "authorization_code"
    assert request["code"] == "fixture-code"
    assert request["client_id"] == state["client_id"]
    assert request["redirect_uri"] == state["redirect_uri"]
    challenge = base64.urlsafe_b64encode(
        hashlib.sha256(request["code_verifier"].encode()).digest()
    ).decode().rstrip("=")
    assert challenge == state["code_challenge"]
    state["exchange"] = request
    if os.environ.get("MOCK_EXCHANGE_FAILURE"):
        print(json.dumps({"error": "invalid_grant"}), end="")
    else:
        claims = {
            "iss": state["issuer"],
            "azp": state["client_id"],
            "aud": [state["audience"]],
            "exp": int(time.time()) + 3600,
        }
        mutation = os.environ.get("MOCK_BAD_BINDING")
        if mutation == "issuer":
            claims["iss"] = "https://other.example/"
        elif mutation == "client":
            claims["azp"] = "ordinary-client"
        elif mutation == "audience":
            claims["aud"] = ["https://wrong-audience.example/"]
        elif mutation == "expiry":
            claims["exp"] = int(time.time()) - 1
        elif mutation == "expiry-type":
            claims["exp"] = "tomorrow"
        elif mutation == "expiry-bool":
            claims["exp"] = True
        elif mutation == "expiry-nan":
            claims["exp"] = float("nan")
        elif mutation == "expiry-infinity":
            claims["exp"] = float("inf")
        elif mutation == "claims-type":
            claims = []
        if os.environ.get("MOCK_STRING_AUDIENCE"):
            claims["aud"] = state["audience"]
        encoded = base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip("=")
        token = "e30." + encoded + ".fixture-signature"
        if mutation == "shape":
            token = token.rsplit(".", 1)[0]
        elif mutation == "json":
            token = "e30.bm90LWpzb24.fixture-signature"
        if os.environ.get("MOCK_OPAQUE_TOKEN"):
            token = "ordinary-opaque-token"
        print(json.dumps({"access_token": token, "expires_in": 3600}), end="")
else:
    raise SystemExit("Unexpected request")
state_file.write_text(json.dumps(state))
"""

MOCK_KUBECTL = r"""#!/usr/bin/env python3
import json
import os
from pathlib import Path
import subprocess
import sys

args = sys.argv[1:]
root = Path(os.environ["MOCK_ROOT"])
(root / "kubectl.json").write_text(json.dumps(args))
if os.environ.get("MOCK_KUBE_FAILURE"):
    raise SystemExit("No pod access")
index = args.index("node")
environment = dict(os.environ)
environment["PCC_AUTH0_CLIENT_ID"] = (
    "wrong-client" if os.environ.get("MOCK_WRONG_POD") else args[-1]
)
environment["PCC_AUTH0_CLIENT_SECRET"] = "fixture-pod-secret"
if os.environ.get("MOCK_EMPTY_POD_SECRET"):
    environment["PCC_AUTH0_CLIENT_SECRET"] = ""
result = subprocess.run(["node", *args[index + 1:]], env=environment)
raise SystemExit(result.returncode)
"""


class TokenHelperTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix="easycla-token-test-")
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.script = self.root / "get_auth0_token.sh"
        shutil.copyfile(Path(__file__).with_name("get_auth0_token.sh"), self.script)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        for name, source in (("curl", MOCK_CURL), ("kubectl", MOCK_KUBECTL)):
            executable = self.bin / name
            executable.write_text(source)
            executable.chmod(0o700)
        self.environment = dict(
            os.environ,
            PATH=str(self.bin) + os.pathsep + os.environ["PATH"],
            MOCK_ROOT=str(self.root),
        )
        self.environment.pop("DEBUG", None)
        for key in list(self.environment):
            if key.startswith("MOCK_") and key != "MOCK_ROOT":
                del self.environment[key]
        self.password = "fixture password '$value"
        self.file_secret = "fixture-client-secret"
        self.client_id_files()

    def client_id_files(self):
        for stage in ("dev", "prod"):
            for suffix in ("", "-azp"):
                client_file = self.root / f"auth0-{stage}{suffix}-client-id.secret"
                client_file.write_text(f"fixture-{stage}{suffix}-client\n")
                client_file.chmod(0o600)

    def credentials(self, stage="dev", **extra):
        values = {"AUTH0_USERNAME": "fixture-user", "AUTH0_PASSWORD": self.password, **extra}
        target = self.root / f"auth0-{stage}.secret"
        target.write_text("".join(f"{key}={value}\n" for key, value in values.items()))
        target.chmod(0o600)

    def run_helper(self, *args, **environment):
        return subprocess.run(
            ["bash", str(self.script), *args],
            env={**self.environment, **environment},
            capture_output=True,
            text=True,
            timeout=30,
        )

    def state(self):
        return json.loads((self.root / "state.json").read_text())

    def assert_success(self, result, stage="dev", mode="non-azp"):
        self.assertEqual(result.returncode, 0, result.stderr)
        suffix = "-azp" if mode == "azp" else ""
        token_file = self.root / f"auth0-{stage}{suffix}.token.secret"
        self.assertEqual(token_file.read_text(), result.stdout)
        self.assertEqual(token_file.stat().st_mode & 0o777, 0o600)
        self.assertEqual(len(result.stdout.strip().splitlines()), 1)
        for secret in (self.password, self.file_secret, "fixture-pod-secret"):
            self.assertNotIn(secret, result.stdout)
            self.assertNotIn(secret, result.stderr)
        requests = [
            json.loads(line) for line in (self.root / "requests.jsonl").read_text().splitlines()
        ]
        for request in requests:
            for secret in (self.password, self.file_secret, "fixture-pod-secret"):
                self.assertNotIn(secret, " ".join(request["args"]))
        self.assertEqual(self.state()["login"]["password"], self.password)

    def test_existing_defaults_and_explicit_non_azp(self):
        for stage in ("dev", "prod"):
            with self.subTest(stage=stage):
                self.credentials(stage, AUTH0_AZP_CLIENT_SECRET=self.file_secret)
                result = self.run_helper(stage, "non-azp")
                self.assert_success(result, stage)
                self.assertEqual(self.state()["client_id"], f"fixture-{stage}-client")
                self.assertEqual(self.state()["redirect_uri"], "http://localhost:55001/callback")
                self.assertNotIn("client_secret", self.state()["exchange"])
                self.assertFalse((self.root / "kubectl.json").exists())
        self.credentials()
        self.assert_success(self.run_helper())

    def test_missing_client_id_file_fails_before_any_network_or_pod_access(self):
        for mode, suffix in (("non-azp", ""), ("azp", "-azp")):
            with self.subTest(mode=mode):
                self.client_id_files()
                self.credentials(AUTH0_AZP_CLIENT_SECRET=self.file_secret)
                (self.root / f"auth0-dev{suffix}-client-id.secret").unlink()
                result = self.run_helper("dev", mode)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, "")
                self.assertIn(f"auth0-dev{suffix}-client-id.secret", result.stderr)
                self.assertNotIn(self.password, result.stderr)
                self.assertFalse((self.root / "requests.jsonl").exists())
                self.assertFalse((self.root / "kubectl.json").exists())
                self.assertFalse((self.root / f"auth0-dev{suffix}.token.secret").exists())

    def test_only_the_selected_client_id_file_is_needed(self):
        for stage in ("dev", "prod"):
            with self.subTest(mode="azp", stage=stage):
                self.client_id_files()
                (self.root / f"auth0-{stage}-client-id.secret").unlink()
                self.credentials(stage, AUTH0_AZP_CLIENT_SECRET=self.file_secret)
                self.assert_success(self.run_helper(stage, "azp"), stage, "azp")
                self.assertEqual(self.state()["client_id"], f"fixture-{stage}-azp-client")
        with self.subTest(mode="non-azp"):
            self.client_id_files()
            (self.root / "auth0-dev-azp-client-id.secret").unlink()
            self.credentials()
            self.assert_success(self.run_helper("dev"))
            self.assertEqual(self.state()["client_id"], "fixture-dev-client")
        with self.subTest(mode="non-azp", override=True):
            self.client_id_files()
            (self.root / "auth0-dev-client-id.secret").unlink()
            self.credentials(AUTH0_CLIENT_ID="ordinary-override")
            self.assert_success(self.run_helper("dev"))
            self.assertEqual(self.state()["client_id"], "ordinary-override")

    def test_no_client_ids_in_tracked_files(self):
        here = Path(__file__).resolve().parent
        for name in ("get_auth0_token.sh", "test_get_auth0_token.py", "auth0.secret.example"):
            with self.subTest(file=name):
                self.assertNotRegex((here / name).read_text(), r"\b[A-Za-z0-9]{32}\b")
        script = (here / "get_auth0_token.sh").read_text()
        for name in ("dev", "prod", "dev-azp", "prod-azp"):
            self.assertIn(f'"$(cat "$SCRIPT_DIR/auth0-{name}-client-id.secret")"', script)

    def test_ordinary_overrides_remain_literal(self):
        self.credentials(
            AUTH0_DOMAIN="alternate.example",
            AUTH0_CLIENT_ID="ordinary-override",
            AUTH0_AUDIENCE="https://api.example/audience",
            AUTH0_TENANT="literal '$value tenant",
            AUTH0_REDIRECT_URI="http://localhost:55002/callback",
        )
        self.assert_success(self.run_helper("dev"))
        self.assertEqual(self.state()["client_id"], "ordinary-override")
        self.assertEqual(self.state()["login"]["tenant"], "literal '$value tenant")
        self.assertEqual(self.state()["issuer"], "https://alternate.example/")

    def test_ordinary_opaque_output_is_unchanged(self):
        self.credentials()
        result = self.run_helper("dev", MOCK_OPAQUE_TOKEN="1")
        self.assert_success(result)
        self.assertEqual(result.stdout.strip(), "ordinary-opaque-token")

    def test_azp_secret_file_and_stage_bindings(self):
        for stage, redirect in (
            ("dev", "https://app.dev.lfx.dev/callback"),
            ("prod", "https://app.lfx.dev/callback"),
        ):
            with self.subTest(stage=stage):
                self.credentials(
                    stage, AUTH0_AZP_CLIENT_SECRET=self.file_secret,
                    AUTH0_CLIENT_ID="ordinary-override", AUTH0_DOMAIN="ignored.example",
                    AUTH0_AUDIENCE="ignored-audience", AUTH0_REDIRECT_URI="http://ignored.example",
                )
                self.assert_success(self.run_helper(stage, "azp"), stage, "azp")
                self.assertEqual(self.state()["client_id"], f"fixture-{stage}-azp-client")
                self.assertEqual(self.state()["redirect_uri"], redirect)
                self.assertEqual(self.state()["exchange"]["client_secret"], self.file_secret)
                self.assertFalse((self.root / "kubectl.json").exists())

    def test_azp_uses_matching_stage_pod(self):
        for stage in ("dev", "prod"):
            with self.subTest(stage=stage):
                self.credentials(stage)
                self.assert_success(self.run_helper(stage, "azp"), stage, "azp")
                args = json.loads((self.root / "kubectl.json").read_text())
                self.assertEqual(args[args.index("--context") + 1], "lfx-" + stage)
                self.assertEqual(args[args.index("--namespace") + 1], "ui")
                self.assertEqual(self.state()["exchange"]["client_secret"], "fixture-pod-secret")

    def test_no_fallback_when_pod_access_or_configuration_is_wrong(self):
        self.credentials()
        for flag in ("MOCK_KUBE_FAILURE", "MOCK_WRONG_POD", "MOCK_EMPTY_POD_SECRET"):
            with self.subTest(flag=flag):
                result = self.run_helper("dev", "azp", **{flag: "1"})
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("azp requires", result.stderr)
                self.assertEqual(result.stdout, "")
                self.assertFalse((self.root / "requests.jsonl").exists())
                self.assertFalse((self.root / "auth0-dev-azp.token.secret").exists())

    def test_azp_rejects_invalid_token_bindings(self):
        self.credentials(AUTH0_AZP_CLIENT_SECRET=self.file_secret)
        for mutation in ("issuer", "client", "audience", "expiry", "expiry-type", "expiry-bool", "expiry-nan",
                         "expiry-infinity", "claims-type", "shape", "json"):
            with self.subTest(mutation=mutation):
                result = self.run_helper("dev", "azp", MOCK_BAD_BINDING=mutation)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, "")
                self.assertIn("error: azp token", result.stderr)
                self.assertFalse((self.root / "auth0-dev-azp.token.secret").exists())

    def test_azp_accepts_string_audience(self):
        self.credentials(AUTH0_AZP_CLIENT_SECRET=self.file_secret)
        self.assert_success(self.run_helper("dev", "azp", MOCK_STRING_AUDIENCE="1"), mode="azp")

    def test_modes_keep_independent_token_files(self):
        self.credentials(AUTH0_AZP_CLIENT_SECRET=self.file_secret)
        ordinary = self.run_helper("dev")
        self.assert_success(ordinary)
        trusted = self.run_helper("dev", "azp")
        self.assert_success(trusted, mode="azp")
        self.assertEqual((self.root / "auth0-dev.token.secret").read_text(), ordinary.stdout)
        self.assertNotEqual(ordinary.stdout, trusted.stdout)

    def test_bad_arguments_and_missing_inputs_do_not_contact_provider(self):
        for args in (("unknown",), ("dev", "unknown"), ("dev",)):
            with self.subTest(args=args):
                result = self.run_helper(*args)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, "")
                self.assertFalse((self.root / "requests.jsonl").exists())
        self.credentials(AUTH0_PASSWORD="")
        result = self.run_helper("dev", "azp")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.root / "kubectl.json").exists())

    def test_provider_failures_do_not_write_token(self):
        self.credentials(AUTH0_AZP_CLIENT_SECRET=self.file_secret)
        for flag in ("MOCK_LOGIN_FAILURE", "MOCK_EXCHANGE_FAILURE"):
            with self.subTest(flag=flag):
                result = self.run_helper("dev", "azp", **{flag: "1"})
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, "")
                self.assertFalse((self.root / "auth0-dev-azp.token.secret").exists())
                self.assertNotIn(self.password, result.stderr)
                self.assertNotIn(self.file_secret, result.stderr)


if __name__ == "__main__":
    unittest.main()
