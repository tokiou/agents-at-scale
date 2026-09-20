# Completar rebooking en LangGraph/Python

## Objective

Completar el agente LangGraph/Python para que ejecute el workflow de rebooking con la misma semántica observable que `adk-go`: validación contra estado actual, ejecución atómica y verificación posterior, persistencia PostgreSQL mediante repositorios SQLAlchemy async, un adaptador LLM real compatible con la configuración OpenRouter existente, y composición separada entre dominio, grafo, FastAPI y workers. La implementación deberá incluir tests unitarios y de integración reproducibles, sin modificar el código Go de referencia.

## Relevant Context

- El runtime Python está en `langgraph-python/app`; el dominio airline está en `app/airline/`.
- `app/airline/models.py` ya refleja las tablas y enums de `adk-go/db/schema.sql`; `schemas.py` expone DTOs, pero no existen aún contratos Python para snapshot, validación y resultado de rebooking.
- Los repositorios de lectura (`customer`, `reservation`, `flight`, `travel_credit`, `flight_change`) crean sesiones independientes y no hay repositorio de rebooking.
- `app/airline/service.py` implementa búsqueda de opciones, créditos e historial, pero no implementa `validate_rebooking_selection`, `execute_rebooking` ni `verify_rebooking`.
- El grafo existente implementa fan-out, evaluación, `interrupt` y retry, pero termina en `ready`; no ejecuta ni verifica la mutación.
- `app/main.py` sólo compone PostgreSQL/Redis/RabbitMQ y `JobService`; el consumidor marca el job como completado sin ejecutar el agente. No hay composición de `Service`, grafo, runner/checkpointer ni adaptador LLM.
- `requirements.txt` no declara SDK de LLM ni `httpx`; la configuración raíz y la implementación Go sí definen `OPENROUTER_DEPLOYMENT`, `OPENROUTER_API_KEY` y `OPENROUTER_BASE_URL`. Los prompts Go equivalentes están en `adk-go/internal/agent/prompts/`.
- `adk-go/internal/airline/service.go` y `repository/rebooking.go` son la referencia normativa para reglas, locks, actualización de inventario, crédito, total, segmento, `flight_changes` y verificación.
- No se encontró un directorio de migraciones ni un runner de migraciones Python. El único schema SQL versionable inspeccionado es `adk-go/db/schema.sql`.

## Scope

Incluye:

1. Contratos y reglas Python equivalentes para validar, ejecutar y verificar un rebooking.
2. Repositorios PostgreSQL async y transacción única para la ejecución.
3. Extensión del grafo para `execute_rebooking -> verify_rebooking` y resultados finales.
4. Adaptador real OpenRouter y configuración explícita.
5. Composición de dependencias y ejecución desde FastAPI/jobs sin SQL, broker ni clientes LLM dentro de nodos.
6. Tests unitarios, de workflow, de repositorio/transacción y de integración con servicios controlados.

No incluye cambios al runtime Go, autenticación/autorización, pagos externos, inventario de múltiples segmentos en una misma operación, ni observabilidad nueva más allá de errores/logs necesarios para diagnosticar el flujo.

## Expected Behavior

El flujo completo será:

```text
START -> understand_request -> get_reservation
       -> (search_alternatives || get_travel_credits)
       -> evaluate_options -> ask_confirmation (interrupt)
       -> validate_change
          invalid -> explain_invalid_change -> retry/evaluate_options
          declined -> final(declined)
          valid -> execute_rebooking -> verify_rebooking -> final(completed)
```

La validación previa al `execute_rebooking` no sustituye la validación dentro de la transacción: la transacción deberá volver a leer y validar el estado bloqueado para evitar ejecutar una selección obsoleta.

## Functional Requirements

### Contratos y servicio

1. El módulo `app.airline` SHALL definir tipos Pydantic para `RebookingSelection`, `RebookingSnapshot`, `RebookingValidation` y `RebookingResult`, conservando `UUID`, `datetime`, `Decimal` y enums.
2. `Service.validate_rebooking_selection` SHALL obtener snapshot y crédito a través del repositorio y SHALL reproducir las reglas de `adk-go/internal/airline/service.go`: segmento/reserva modificables, no re-aplicación, vuelo y tarifa objetivo disponibles, ruta y moneda compatibles, asientos suficientes, crédito perteneciente al cliente/no expirado/con saldo/estado válido y monto de crédito exactamente compatible con la diferencia y comisión.
3. Las reglas de dinero SHALL usar `Decimal`; una deuda negativa SHALL quedar en cero, igual que Go.
4. `Service.execute_rebooking` SHALL delegar la operación atómica al repositorio; `Service.verify_rebooking` SHALL delegar la lectura/verificación y SHALL distinguir una verificación fallida de un error de infraestructura.
5. Los errores de reglas SHALL ser identificables por tipos/valores estables y los errores técnicos SHALL conservar su causa; no se convertirán en una decisión `declined`.

### Repositorios y PostgreSQL

6. `app/airline/repository/rebooking.py` SHALL implementar operaciones nombradas equivalentes a `GetSnapshot`, `Execute` y `Verify`; ningún handler, nodo o servicio HTTP SHALL contener SQL.
7. Snapshot y ejecución SHALL leer el segmento/reserva con `FOR UPDATE`, el vuelo/tarifa objetivo con `FOR UPDATE` y, si corresponde, el crédito con `FOR UPDATE`, usando una misma `AsyncSession` transaccional durante `execute`.
8. La ejecución SHALL, en una única transacción y en este orden lógico verificable, revalidar snapshot, decrementar asientos objetivo de forma condicionada, restaurar asientos originales, actualizar segmento a `CHANGED`, ajustar total, consumir crédito si corresponde, insertar `flight_changes`, hacer commit y devolver el resultado persistido.
9. Cualquier excepción o fallo de una actualización condicionada SHALL provocar rollback y no dejar cambios parciales. El commit SHALL ocurrir sólo después de crear el historial.
10. `verify` SHALL comprobar que el segmento solicitado tiene los nuevos IDs y estado `CHANGED`, y SHALL devolver booking reference y datos del segmento; si no coincide SHALL producir el error equivalente a `ErrRebookingVerification`.
11. Los nombres, columnas, enums y precisión de `langgraph-python/app/airline/models.py` SHALL permanecer compatibles con `adk-go/db/schema.sql`. Si se necesita una migración, SHALL ser versionada y SHALL documentar cómo se aplica; no se crearán tablas incompatibles o duplicadas.

### Grafo y adaptadores

12. `app.agent.nodes` SHALL exponer nodos inyectables `execute_rebooking` y `verify_rebooking`; éstos sólo llamarán puertos de dominio y no accederán a sesiones, Redis, RabbitMQ ni HTTP.
13. La ruta `valid` SHALL pasar a ejecución y verificación; no SHALL reportar éxito antes de ambas operaciones. Fallos técnicos de ejecución/verificación SHALL terminar en `failed`, con nodo y mensaje seguro, y no SHALL reintentarse como selección inválida.
14. La selección validada SHALL viajar al nodo de ejecución y la verificación SHALL usar la misma selección; el grafo SHALL conservar el resultado de ejecución y el resultado final verificable.
15. La pausa `interrupt` SHALL continuar siendo anterior a toda mutación. Resume inválido, selección no ofrecida o rechazo SHALL conservar las reglas de retry/declined ya especificadas.

16. `app/platform/llm/openrouter.py` SHALL implementar un cliente async para `POST {base_url}/chat/completions`, con bearer token, modelo deployment, prompts equivalentes a Go y validación de respuesta HTTP/JSON/choices.
17. El adaptador SHALL producir `RebookingRequest` y `EvaluationResult` estructurados, validar que los IDs evaluados pertenecen a las opciones recibidas y no inventar datos de negocio. Errores de red, HTTP, JSON o respuesta vacía SHALL propagarse como fallos del nodo.
18. La configuración SHALL leer `OPENROUTER_DEPLOYMENT`, `OPENROUTER_API_KEY` y `OPENROUTER_BASE_URL` sin registrar la clave. La construcción del cliente SHALL validar deployment/key/base URL y no abrir conexiones al importar módulos.

### Composición HTTP/jobs

19. `app/main.py` SHALL crear engine, session factory, repositorios, servicio airline, adaptador LLM, grafo compilado y runner/ejecutor en lifespan, y SHALL cerrarlos en orden seguro.
20. Los nodos SHALL recibir sólo puertos; la composición SHALL ser el único lugar que conozca implementaciones concretas.
21. `JobService.consume` SHALL deserializar y validar un contrato de job de agente, invocar el runner con el payload y marcar `completed` sólo después de un resultado terminal exitoso. Excepciones SHALL marcar `failed` y SHALL aplicar la política de `nack` existente; nunca se hará `ack` de éxito prematuro.
22. Los endpoints FastAPI SHALL limitarse a validar request, publicar jobs y consultar/expresar estado; no ejecutarán SQL ni reglas airline. Si se expone reanudación HTTP, SHALL publicar una continuación identificable por thread/session ID y no crear un grafo por request.
23. La cola y las claves Redis SHALL conservar el nombre configurable existente y no mezclar estados de jobs con estado interno del dominio.

### Tests

24. Los tests SHALL seguir `unittest`/`IsolatedAsyncioTestCase` existente y usar fakes para LLM, airline y broker donde no se requiera PostgreSQL.
25. SHALL cubrir reglas de validación (cada rechazo relevante), cálculo Decimal, selección con/sin crédito, ejecución exitosa y verificación exitosa.
26. SHALL cubrir rollback ante fallo en inventario, crédito, actualización, inserción o commit, y concurrencia suficiente para demostrar que los locks/actualizaciones condicionadas no permiten sobreventa.
27. SHALL cubrir el grafo completo: interrupt sin mutación, declined, invalid/retry, valid que ejecuta y verifica, error técnico que no se convierte en éxito.
28. SHALL cubrir el adaptador OpenRouter con HTTP controlado: request/modelo/auth/payload, respuesta válida, HTTP no-2xx, JSON inválido y choices vacías.
29. SHALL cubrir composición/job: el job no se marca completado antes del runner, errores producen estado failed y nack, y el lifespan inyecta dependencias sin efectos al importar.

## Non-Functional Requirements

- Todas las operaciones I/O Python SHALL ser async y respetar el pool SQLAlchemy existente.
- La transacción SHALL ser corta, sin llamadas LLM, Redis, RabbitMQ ni HTTP externo dentro de ella.
- Los logs no SHALL incluir API keys, payloads completos con datos sensibles ni credenciales.
- El comportamiento Go/Python SHALL ser comparable en estados, rutas, cálculos monetarios y efectos en las tablas.

## Affected Components

- `langgraph-python/app/airline/models.py`, `schemas.py`, `service.py` y `repository/`.
- `langgraph-python/app/agent/state.py`, `graph.py` y `agent/nodes/`.
- `langgraph-python/app/platform/llm/`, `config.py`, `requirements.txt` y `.env.example`.
- `langgraph-python/app/main.py`, `app/jobs/service.py` y contratos/router de jobs.
- `langgraph-python/tests/` y, si se adopta migración Python, un directorio de migraciones documentado.

## Constraints

- `adk-go/db/schema.sql`, `adk-go/db/query/airline.sql` y `adk-go/internal/airline/` son referencia; no editar sus generados ni copiar SQL a handlers/nodos.
- Python SHALL usar SQLAlchemy 2.x async con `asyncpg`; no añadir raw SQL para consultas airline fuera del repositorio/migración.
- El grafo SHALL seguir compilable con `langgraph>=0.3,<1.0` y usar checkpointer sólo desde composición.
- No se añadirán credenciales al repositorio ni se usarán valores de `.env` como secretos por defecto.

## Edge Cases

- Segmento inexistente, reserva no confirmada, segmento `CANCELLED`/`FLOWN`, selección ya aplicada.
- Vuelo/tarifa desaparecidos, vuelo no `SCHEDULED`, ruta/moneda incompatibles, asientos insuficientes por carrera.
- Crédito inexistente, de otro cliente, expirado, agotado, moneda incorrecta, monto cero/negativo o distinto de la deuda.
- Error después de cada mutación, cancelación del contexto, commit fallido, verificación ausente o inconsistente.
- Job duplicado, payload inválido, reanudación sin checkpoint y worker detenido durante procesamiento.

## Error Handling

Los errores de negocio de validación se expondrán como resultado inválido/retry; rechazo explícito será terminal `declined`. Errores de infraestructura, LLM, transacción, ejecución o verificación serán `failed`, conservarán etapa y causa encadenada para logs, no harán ack exitoso y no dejarán mutaciones parciales.

## Acceptance Criteria

- Una selección confirmada sólo termina como éxito después de commit y verificación positiva.
- Dos ejecuciones concurrentes no consumen más asientos/crédito de los disponibles y toda ejecución fallida deja la base igual.
- El estado final contiene resultado de ejecución/verificación y es serializable por LangGraph.
- OpenRouter real puede configurarse sólo con las variables documentadas y sus errores son comprobables.
- Un job ejecuta el grafo real y su estado Redis/RabbitMQ refleja el resultado real.
- `make test`, `python -m unittest discover -s tests -p 'test_*.py'` y los checks Python documentados pasan sin secretos; los tests PostgreSQL usan una base efímera/controlada.

## Test Scenarios

1. Rebooking confirmado con saldo suficiente: commit, `flight_changes`, asientos, total y verify correctos.
2. Rebooking con diferencia negativa y sin crédito: total no baja de forma incorrecta y verify es correcto.
3. Cada rechazo de `ValidateRebookingSelection` no muta tablas.
4. Dos ejecuciones simultáneas al último asiento: una gana y la otra falla/rollback.
5. Fallo al consumir crédito o insertar historial: segmento, total, inventario y crédito se restauran.
6. Grafo pausa antes de ejecución y resume válido alcanza `completed`.
7. Grafo declined/invalid/técnico alcanza estados distintos y no confunde ack/éxito.
8. OpenRouter devuelve respuesta válida, error HTTP, JSON inválido y respuesta vacía.
9. Job válido espera al runner; job inválido o fallido no se ackea como completado.

## Out of Scope

- Cambiar la semántica o implementar el agente Go.
- Migrar a otro proveedor LLM, añadir tool-calling no requerido, pagos reales o autenticación.
- Rediseñar Redis/RabbitMQ, introducir un sistema de migraciones completo sin decisión explícita, o soportar rebooking multi-segmento atómico.

## Assumptions

- OpenRouter es el proveedor Python previsto porque es el único proveedor concreto configurado y el adaptador Go de referencia usa su API compatible con OpenAI.
- La operación de ejecución corresponde a un solo `reservation_segment_id`, como en Go.
- `adk-go/db/schema.sql` es la fuente de verdad del esquema actual.
- El límite de retry existente (por defecto 2) se conserva salvo decisión de producto.

## Critical Decisions / Ambiguities

1. **Migraciones:** no existe runner ni carpeta Python. Decidir antes de implementar si `adk-go/db/schema.sql` seguirá aplicándose externamente, si se añadirá una copia/versionado Python, o si se incorpora Alembic. La especificación exige compatibilidad, pero no fija herramienta.
2. **HTTP de conversación/resume:** sólo existe `POST /jobs` y el job actual no tiene contrato de agente. Confirmar si esta entrega debe añadir endpoints de inicio/consulta/resume o sólo hacer funcionar el worker; el requisito de jobs permite el segundo, pero no define API de conversación.
3. **Persistencia del checkpoint:** el grafo acepta checkpointer y los jobs necesitan un `thread_id`, pero no hay tabla/servicio de checkpoint Python. Confirmar si se usa memoria sólo para tests en esta entrega o un checkpointer persistente; no debe inventarse una tabla.
4. **SDK LLM:** no hay cliente OpenRouter declarado en `requirements.txt`. La decisión recomendada es declarar `httpx` directamente y construir un adaptador pequeño; si no se autoriza añadir dependencia, debe aprobarse explícitamente la alternativa.
