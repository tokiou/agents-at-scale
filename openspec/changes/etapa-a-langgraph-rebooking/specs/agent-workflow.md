# Delta spec: workflow de rebooking LangGraph/Python (Etapa A)

## Added Requirements

El cambio añade al paquete `langgraph-python/app/agent` un contrato de estado compartido, nodos con colaboraciones inyectables y un grafo de rebooking con fan-out/fan-in, pausa humana y routing `valid`, `invalid`, `retry` y `declined`, conforme a `proposal.md`.

La implementación debe ser testeable sin infraestructura externa. `valid` termina en un resultado preparado para ejecución futura; no implica persistir ni aplicar el cambio de vuelo.

## Compatibility Notes

Los contratos deben alinearse con `app.airline.schemas`, `app.airline.service.Service` y el workflow de referencia en `adk-go/internal/agent/`. La integración de FastAPI, jobs y checkpointing queda para etapas posteriores.
