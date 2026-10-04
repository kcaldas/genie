# Native Google API proxy

Set `GENIE_GOOGLE_BASE_URL` and `GENIE_GOOGLE_AUTH_TOKEN` to use an authenticated
proxy with either `GENAI_BACKEND=gemini` or `GENAI_BACKEND=vertex`. Genie keeps the
selected backend's native request and response format. The proxy receives a
Bearer credential and `X-Genie-Client: genie`; no Google credential is required
on the client. HTTPS is required except on loopback for local development.

The proxy must implement native model generation, streaming and token counting
paths and supply upstream authentication. Vertex uses the configured project
and location in its path (project defaults to `proxy` when absent); the proxy
should resolve these to its own authorized project and location.

Explicit proxy mode never falls back to a direct Google backend. Redirects are
not followed. Without these variables the existing direct configuration is
unchanged.
