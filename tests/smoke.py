"""Framework neutral smoke tests for go-base.

Concretisation of tests/oracle.feature. These tests talk to a running container
over HTTP only - they never import the application, inspect its source, or run
its unit tests, so they are valid against any framework implementation.

Run:
    BASE_URL=http://localhost:3000 AUTH_JWT_SECRET=<secret> pytest -q tests/smoke.py
"""

import base64
import hashlib
import hmac
import json
import os
import time

import pytest
import requests

BASE_URL = os.environ.get("BASE_URL", "http://localhost:3000").rstrip("/")
JWT_SECRET = os.environ["AUTH_JWT_SECRET"]

TIMEOUT = 10


def _b64(raw: bytes) -> str:
    return base64.urlsafe_b64encode(raw).decode().rstrip("=")


def mint_jwt(claims: dict, secret: str = None) -> str:
    """Sign an HS256 access token.

    The oracle mints its own token rather than scraping the login token from the
    container log: the passwordless login token is delivered by email only, so
    it never crosses the HTTP boundary this oracle is allowed to observe.
    """
    secret = JWT_SECRET if secret is None else secret
    now = int(time.time())
    payload = {"iat": now, "exp": now + 900, **claims}
    header = _b64(json.dumps({"alg": "HS256", "typ": "JWT"}, separators=(",", ":")).encode())
    body = _b64(json.dumps(payload, separators=(",", ":")).encode())
    signing_input = f"{header}.{body}".encode()
    signature = hmac.new(secret.encode(), signing_input, hashlib.sha256).digest()
    return f"{header}.{body}.{_b64(signature)}"


# marker string from public/index.html, used to prove the SPA index was served
SPA_MARKER = "Replace me with your PWA"

ADMIN_JWT = {"id": 1, "sub": "Admin Example", "roles": ["admin"]}
USER_JWT = {"id": 2, "sub": "User Example", "roles": ["user"]}


def auth_header(claims: dict, secret: str = None) -> dict:
    return {"Authorization": f"Bearer {mint_jwt(claims, secret)}"}


def get(path: str, **kwargs):
    return requests.get(BASE_URL + path, timeout=TIMEOUT, **kwargs)


def post(path: str, **kwargs):
    return requests.post(BASE_URL + path, timeout=TIMEOUT, **kwargs)


# --- readiness and static -------------------------------------------------

def test_healthz_returns_ok():
    r = get("/healthz")
    assert r.status_code == 200
    assert r.text == "ok"


def test_root_serves_the_spa():
    r = get("/")
    assert r.status_code == 200
    assert SPA_MARKER in r.text


def test_unknown_path_falls_back_to_the_spa():
    r = get("/some/unknown/path")
    assert r.status_code == 200, "unknown paths must fall back to the SPA, not 404"
    assert SPA_MARKER in r.text
    assert r.text == get("/").text, "the fallback must serve the same index page"


# --- authentication boundary ----------------------------------------------

def test_protected_route_without_token_is_unauthorized():
    r = get("/api/account")
    assert r.status_code == 401
    assert r.json() == {"status": "Unauthorized", "error": "token unauthorized"}


def test_protected_route_with_foreign_signature_is_unauthorized():
    r = get("/api/account", headers=auth_header(ADMIN_JWT, secret="x" * 64))
    assert r.status_code == 401


# --- passwordless login ---------------------------------------------------

def test_login_with_registered_email_is_accepted():
    r = post("/auth/login", json={"email": "admin@example.com"})
    assert r.status_code == 200
    assert r.json() == {}


def test_login_with_unregistered_email_is_rejected():
    r = post("/auth/login", json={"email": "nobody@example.com"})
    assert r.status_code == 401
    assert r.json()["error"] == "email not registered"


def test_login_with_malformed_email_is_rejected():
    r = post("/auth/login", json={"email": "not-an-email"})
    assert r.status_code == 401
    assert r.json()["error"] == "invalid email address"


# --- authenticated application resources ----------------------------------

def test_account_returns_the_authenticated_account():
    r = get("/api/account", headers=auth_header(ADMIN_JWT))
    assert r.status_code == 200
    body = r.json()
    assert body["id"] == 1
    assert body["email"] == "admin@example.com"
    assert body["name"] == "Admin Example"
    assert body["active"] is True
    assert body["roles"] == ["admin"]


def test_profile_returns_the_default_theme():
    r = get("/api/profile", headers=auth_header(ADMIN_JWT))
    assert r.status_code == 200
    assert r.json()["theme"] == "default"


# --- administration resources ---------------------------------------------

def test_admin_lists_all_accounts():
    r = get("/admin/accounts", headers=auth_header(ADMIN_JWT))
    assert r.status_code == 200
    body = r.json()
    assert body["count"] == 2
    assert {a["email"] for a in body["accounts"]} == {
        "admin@example.com",
        "user@example.com",
    }


def test_admin_index_greets():
    r = get("/admin/", headers=auth_header(ADMIN_JWT))
    assert r.status_code == 200
    assert r.text == "Hello Admin"


def test_non_admin_is_forbidden_from_admin_routes():
    r = get("/admin/accounts", headers=auth_header(USER_JWT))
    assert r.status_code == 403


# --- route shape ----------------------------------------------------------

@pytest.mark.parametrize("path", ["/api/account", "/api/account/", "/api/profile", "/api/profile/"])
def test_resource_paths_answer_with_and_without_trailing_slash(path):
    r = get(path, headers=auth_header(ADMIN_JWT))
    assert r.status_code == 200, f"{path} must be routable"
