# Optional Ollama-compatible provider

The `ai-local` Compose profile starts an Ollama-compatible HTTP endpoint without enabling AI in the CRM itself:

```sh
docker compose --profile ai-local up -d ollama
```

Explicitly set `AI_PROVIDER=ollama` and `AI_BASE_URL=http://ollama:11434` only after selecting and pulling a model. The application remains `disabled` by default, and no CRM data is sent merely because the optional container is running.

Model selection is intentionally not baked into the image or startup command: model downloads are large, hardware-dependent, and require an explicit operator decision.
