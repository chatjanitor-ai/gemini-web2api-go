# gemini-web2api-go

Fork of `zexadev/gemini-web2api-go` maintained at `chatjanitor-ai/gemini-web2api-go`.
The upstream remote remains `origin`; push organization changes to `fork`.

- Internal Docker address: `http://gemini-web2api:8083/v1` on the `proxy` network.
- OpenAI-compatible routes for 9router: `POST /v1/images/generations` and `POST /v1/images/edits`.
- Image model: `gemini-image`; response is `data[0].b64_json`. Requires a Google cookie account in the server's persistent SQLite database.
- Credentials: server-only `API_KEY` and `ADMIN_TOKEN` in `/opt/deployment/gemini-web2api-go/.env`; never commit them or cookie values.
- CI source of truth: `janitor-infra/ci-templates/apps/gemini-web2api-go.build.yml`. Deploy manifest: `janitor-infra/gemini-web2api-go/docker-compose.yml`.
