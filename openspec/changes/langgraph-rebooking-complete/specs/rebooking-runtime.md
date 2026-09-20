# Delta spec: ejecución completa del rebooking LangGraph/Python

Esta delta implementa `validate_change -> execute_rebooking -> verify_rebooking` y la persistencia atómica equivalente a `adk-go`. Aplica junto con `proposal.md` y amplía la Etapa A existente; no la reemplaza.

## Requirements

- El grafo SHALL conservar la evaluación y la selección entre interrupt/resume y validación.
- La validación SHALL ejecutarse una vez antes de mutar para feedback inmediato y otra vez dentro de la transacción con filas bloqueadas.
- El repositorio SHALL usar una sesión/transacción compartida para snapshot, updates, consumo de crédito e inserción de historial.
- La ruta `valid` SHALL continuar sólo si ejecución y verificación retornan datos coherentes; la ruta técnica fallida SHALL terminar `failed` sin retry de negocio.
- El contrato de job SHALL transportar un identificador de thread y la entrada/resume necesaria, y el consumidor SHALL ackear sólo después del resultado terminal exitoso.

## Verification

Se verificará con tests de reglas, rollback, carrera de inventario, grafo, adaptador HTTP y consumidor descritos en `proposal.md`.
