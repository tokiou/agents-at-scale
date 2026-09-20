# Delta spec: LLM real y composición

## Requirements

- SHALL existir un adaptador async OpenRouter basado en la configuración `OPENROUTER_*` y prompts equivalentes a Go.
- SHALL validar entradas/salidas estructuradas y ocultar credenciales de errores/logs.
- `main.py` SHALL componer engine, session factory, repositorios, servicio, adaptador, grafo y jobs durante lifespan; módulos de dominio/nodos SHALL permanecer libres de composición concreta.
- El consumidor SHALL ejecutar el grafo real y reflejar estados reales en Redis/RabbitMQ.
- La estrategia de migración y el alcance de endpoints de resume SHALL quedar documentados conforme a las decisiones críticas de la propuesta antes de codificar.
