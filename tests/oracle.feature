Feature: go-base HTTP boundary
  The externally observable behaviour of the go-base API. These scenarios are
  framework neutral: they describe what a client sees, never how the server is
  built. They must hold identically before and after the chi -> echo migration.

  Scenario: The service reports readiness
    Given the service is running
    When a client requests GET /healthz
    Then the response status is 200
    And the response body is exactly "ok"

  Scenario: The single page application is served at the root
    Given the service is running
    When a client requests GET /
    Then the response status is 200
    And the response body contains "Replace me with your PWA"

  Scenario: Unknown paths fall back to the single page application
    Given the service is running
    When a client requests GET /some/unknown/path
    Then the response status is 200
    And the response body contains "Replace me with your PWA"
    And the response body is identical to the one served at /

  Scenario: Protected resources reject requests without a token
    Given the service is running
    When a client requests GET /api/account without an Authorization header
    Then the response status is 401
    And the response body is {"status": "Unauthorized", "error": "token unauthorized"}

  Scenario: Protected resources reject a token signed with the wrong key
    Given the service is running
    When a client requests GET /api/account with a token signed by an unknown key
    Then the response status is 401

  Scenario: A login request for a registered account is accepted
    Given an account "admin@example.com" exists and may log in
    When a client posts {"email": "admin@example.com"} to /auth/login
    Then the response status is 200
    And the response body is exactly {}

  Scenario: A login request for an unregistered address is rejected
    Given no account "nobody@example.com" exists
    When a client posts {"email": "nobody@example.com"} to /auth/login
    Then the response status is 401
    And the response body reports the error "email not registered"

  Scenario: A login request with a malformed address is rejected
    Given the service is running
    When a client posts {"email": "not-an-email"} to /auth/login
    Then the response status is 401
    And the response body reports the error "invalid email address"

  Scenario: An authenticated account reads its own record
    Given an authenticated account with id 1
    When a client requests GET /api/account
    Then the response status is 200
    And the response body reports email "admin@example.com" and roles ["admin"]

  Scenario: An authenticated account reads its own profile
    Given an authenticated account with id 1
    When a client requests GET /api/profile
    Then the response status is 200
    And the response body reports theme "default"

  Scenario: An administrator lists all accounts
    Given an authenticated account with id 1 holding the admin role
    When a client requests GET /admin/accounts
    Then the response status is 200
    And the response body reports a count of 2 accounts

  Scenario: An administrator reaches the admin index
    Given an authenticated account with id 1 holding the admin role
    When a client requests GET /admin/
    Then the response status is 200
    And the response body is exactly "Hello Admin"

  Scenario: A non administrator is refused administration routes
    Given an authenticated account with id 2 holding only the user role
    When a client requests GET /admin/accounts
    Then the response status is 403

  Scenario: Resource paths answer with and without a trailing slash
    Given an authenticated account with id 1
    When a client requests GET /api/account and GET /api/account/
    Then both responses have status 200 and identical bodies
