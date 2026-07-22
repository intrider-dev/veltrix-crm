# Optional local SMTP capture

Start the capture service and point the application at it explicitly:

```sh
docker compose --profile mail up -d mailpit
SMTP_URL=smtp://mailpit:1025 docker compose up -d app
```

Mailpit's local web interface is available at `http://localhost:8025` with the default port mapping. No SMTP service is required for the base CRM profile.
