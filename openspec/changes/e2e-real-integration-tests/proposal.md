# Pruebas de integración end-to-end reales para ambos runtimes

## Objective

Definir y habilitar una suite reproducible de pruebas end-to-end que ejecute el flujo real de rebooking de `adk-go` y `langgraph-python` desde HTTP hasta el worker/agente y PostgreSQL, usando instancias reales de PostgreSQL, Redis y RabbitMQ, datos deterministas para varios usuarios, y un LLM controlado localmente que no contacte OpenRouter. La suite deberá demostrar tanto el resultado observable del job como la mutación persistida y deberá cubrir la pausa de confirmación y su reanudación cuando el runtime la exponga.

## Relevant Context

- Ambos runtimes exponen `POST /airline/chat`, publican un job en RabbitMQ y usan Redis para el estado del job. Go valida `user_id`/`session_id` y Python valida `payload.thread_id` junto con `input`/`resume`.
- `adk-go/internal/app/app.go` compone PostgreSQL, OpenRouter, sesiones ADK en memoria, Redis, RabbitMQ y el consumidor. `langgraph-python/app/main.py` compone SQLAlchemy, `AsyncPostgresSaver`, OpenRouter, Redis, RabbitMQ y `JobService` en el lifespan.
- El rebooking real consulta y modifica las tablas de `db/schema.sql`; Python ya tiene `db/schema.sql` y `db/mock_data.sql`, mientras el Compose de Go no monta actualmente schema ni fixtures.
- Los Compose existentes ya proporcionan PostgreSQL, Redis y RabbitMQ saludables, pero usan volúmenes persistentes y no ofrecen por sí solos una base limpia precargada para tests.
- Redis sólo expone actualmente estados `job:<id>:status`; RabbitMQ usa la cola configurable `agent_jobs` y el consumidor confirma el mensaje después del procesamiento.
- El flujo LangGraph pausa en `interrupt` y acepta `resume` con `thread_id`. Go pausa mediante la herramienta/protocolo ADK de entrada y reanuda con `user_id`, `session_id`, `interrupt_id`, `name` y `payload`.
- Ambos ensamblajes crean directamente un adaptador OpenRouter, por lo que la prueba necesita una seam de composición o un adaptador compatible inyectable. No se debe usar una API key, una cuenta externa ni depender de la disponibilidad de OpenRouter.

## Scope

Incluye:

1. Harness de integración por runtime, con stack real de PostgreSQL, Redis, RabbitMQ y la aplicación/worker real.
2. Inicialización limpia de schema y fixtures deterministas con al menos tres usuarios, reservas independientes, vuelos alternativos, tarifas, créditos y capacidades suficientes para probar éxitos y rechazos.
3. Cliente HTTP que simule al usuario final y sólo interactúe con endpoints públicos del runtime.
4. LLM controlado localmente, determinista y compatible con el contrato que consume cada runtime, con respuestas distintas para extracción de solicitud, evaluación de opciones y escenarios de retry.
5. Polling verificable de estado Redis/RabbitMQ, espera con timeout y diagnóstico de mensajes/logs cuando un job no progresa.
6. Escenarios con confirmación/resume, éxito, rechazo y aislamiento entre usuarios, incluyendo comprobación directa de PostgreSQL después del flujo.
7. Comandos/documentación de ejecución local y en CI sin secretos.

No incluye benchmark de rendimiento, carga masiva, proveedor LLM real, autenticación, pruebas unitarias de reglas ya cubiertas, ni rediseño de la API de negocio.

## Expected Behavior

Cada prueba deberá:

1. Levantar o seleccionar un stack aislado y esperar healthchecks reales de los cuatro servicios relevantes.
2. Aplicar el schema correspondiente y cargar fixtures conocidos; la carga deberá ser idempotente o ejecutarse sobre una base/volumen nuevo.
3. Iniciar la aplicación real con el adaptador LLM controlado, dejando activos el publisher HTTP y el consumidor RabbitMQ.
4. Enviar una solicitud HTTP con identidad y sesión/thread de un usuario fixture.
5. Obtener `202` y un job ID; comprobar primero `published` y finalmente `processing`, `waiting`/`waiting_for_confirmation`, `completed` o `failed` según el caso.
6. Para una pausa, verificar que no exista mutación de rebooking antes del resume; enviar la continuación por HTTP usando el mismo identificador de sesión/thread y los datos de interrupción requeridos; comprobar después el resultado terminal.
7. Consultar PostgreSQL con una conexión de test independiente y comparar filas antes/después: segmento, inventario de tarifas, crédito, total de reserva y `flight_changes`.
8. Limpiar contenedores, volúmenes, colas, claves Redis y datos de prueba aunque falle una aserción.

La prueba no será válida si sustituye RabbitMQ, Redis o PostgreSQL por mocks, si invoca el servicio o el grafo directamente en lugar de HTTP, o si usa un LLM externo.

## Functional Requirements

### Harness y aislamiento

1. Cada runtime SHALL tener una suite de integración separable de sus tests unitarios, ejecutable con un comando explícito (`make test-integration` o equivalente).
2. El harness SHALL poder arrancar servicios con puertos/configuración aislados y SHALL evitar reutilizar volúmenes persistentes de desarrollo; la ejecución paralela de Go y Python SHALL usar proyectos Compose, bases, colas y Redis DB/keys distintos.
3. El harness SHALL esperar readiness real mediante healthcheck/conexión (`pg_isready`/consulta SQL, `PING`, conexión y declaración de cola, y `/health`) antes de enviar tráfico.
4. La suite SHALL aplicar el schema de cada runtime y SHALL cargar fixtures versionados dentro del área de tests/infraestructura. No SHALL depender de datos creados manualmente en un contenedor anterior.
5. Las aserciones SHALL usar un usuario/conexión de PostgreSQL fuera de la aplicación y no SHALL leer estado interno del servicio para inferir mutaciones.

### Fixtures y aislamiento de usuarios

6. Los fixtures SHALL contener al menos tres clientes con IDs estables y reservas/segmentos no compartidos. SHALL existir, como mínimo, un caso con crédito disponible, uno sin crédito y uno con una condición no modificable o sin alternativa válida.
7. Los fixtures SHALL incluir vuelos alternativos y `flight_fares` con capacidades conocidas que permitan calcular exactamente el cambio de asientos, diferencia, comisión y crédito.
8. Cada escenario SHALL declarar su snapshot inicial esperado y SHALL usar IDs de usuario, booking reference, sesión/thread y job únicos o resetear completamente el estado antes de ejecutarse.
9. Una solicitud de un usuario SHALL devolver sólo la reserva, opciones, créditos y mutaciones correspondientes a ese usuario; la suite SHALL incluir una comprobación cruzada que intente usar el booking de otro usuario y verifique que no produce una mutación autorizada por identidad implícita.

### LLM controlado

10. Cada runtime SHALL admitir en modo test un LLM local determinista mediante inyección de dependencia o endpoint compatible local; el modo test SHALL fallar si intenta usar OpenRouter.
11. El LLM controlado SHALL responder de forma estructurada y estable para: extracción de booking/ventana/segmento, evaluación/ranking de opciones y, cuando aplique, primera evaluación inválida seguida de una evaluación válida.
12. El servidor/fake LLM SHALL registrar solicitudes recibidas (sin secretos) y SHALL permitir afirmar que el modelo, prompts/operaciones esperadas y número de llamadas fueron los previstos; no SHALL introducir latencia o aleatoriedad salvo que un escenario lo declare.
13. La suite SHALL probar al menos una respuesta LLM inválida o una secuencia de retry y SHALL comprobar que el error/ruta resultante no muta PostgreSQL.
14. La configuración normal de desarrollo/producción SHALL conservar OpenRouter; el modo de integración SHALL seleccionarse explícitamente por configuración de test y no SHALL modificar `.env` real ni requerir `OPENROUTER_API_KEY`.

### Flujo HTTP, broker y estado

15. El cliente SHALL publicar exclusivamente mediante `POST /airline/chat` y SHALL validar `202`, JSON de respuesta y job ID.
16. La suite SHALL observar Redis usando la clave/convención existente de cada runtime y SHALL afirmar la transición publicada -> procesamiento -> terminal, incluyendo `waiting`/`waiting_for_confirmation` en el caso HITL.
17. La suite SHALL verificar RabbitMQ real: el job debe ser entregado al consumidor de la cola configurada y SHALL quedar confirmado sólo después de que el resultado y la mutación correspondiente hayan terminado. Un fallo SHALL producir el estado de error y la política de rechazo configurada, nunca un éxito falso.
18. El polling SHALL tener timeout finito, backoff acotado y mensajes diagnósticos que incluyan runtime, job, sesión/thread y último estado; no SHALL usar sleeps indefinidos.
19. La suite SHALL cubrir payload inválido, job con identidad/sesión ausente y error del LLM o aplicación, verificando respuesta HTTP apropiada o estado `failed`, ausencia de mutación y ausencia de ack exitoso.

### Confirmación, resume y mutación

20. Una solicitud válida SHALL detenerse antes de ejecutar el cambio y SHALL quedar esperando confirmación; la consulta de PostgreSQL en ese punto SHALL mostrar segmento, inventario, crédito y `flight_changes` sin cambios.
21. El test SHALL extraer de la respuesta/eventos/logs o del contrato de test los datos necesarios para construir un resume válido, y SHALL enviarlo por el mecanismo HTTP soportado por el runtime con el mismo usuario y sesión/thread.
22. Un resume confirmado SHALL terminar sólo después de una verificación positiva y SHALL producir exactamente una fila `flight_changes`, actualizar el segmento al vuelo/tarifa seleccionados, ajustar inventario y actualizar total/crédito conforme a las reglas existentes.
23. Un resume declinado, una selección no ofrecida, una selección de otro segmento o un resume mal formado SHALL terminar en `declined`, `failed` o retry según el contrato del runtime, y SHALL dejar PostgreSQL igual al snapshot inicial.
24. Las comprobaciones de mutación SHALL comparar, como mínimo, IDs anterior/nuevo, estados, `available_seats`, `remaining_amount`, `status`, `total_amount`, conteo y contenido de `flight_changes`, y timestamps sólo con tolerancia explícita.
25. Los escenarios de Go y Python SHALL ser semánticamente equivalentes: mismo fixture lógico, intención del usuario controlada equivalente, misma selección final y mismas invariantes de base, aunque difieran los nombres exactos del estado (`waiting` frente a `waiting_for_confirmation`).

## Non-Functional Requirements

- Los tests SHALL ser deterministas, repetibles y capaces de ejecutarse sin red externa ni credenciales.
- El cleanup SHALL ejecutarse en `finally`/defer y no SHALL borrar bases fuera del proyecto aislado.
- Los secretos, tokens y payloads sensibles no SHALL aparecer en logs ni reportes.
- Los timeouts SHALL distinguir fallo de infraestructura de fallo funcional y SHALL conservar logs de aplicación, fake LLM, broker y base para diagnóstico.
- La suite SHALL evitar carreras introducidas por el propio test: deberá esperar por estado observable y usar transacciones/conexiones separadas al verificar PostgreSQL.

## Affected Components

- `adk-go/docker-compose.yml`, `adk-go/Makefile`, `adk-go/db/` y configuración de integración.
- `langgraph-python/docker-compose.yml`, `langgraph-python/Makefile`, `langgraph-python/db/` y configuración de integración.
- Nuevos directorios de harness/fixtures, previsiblemente `adk-go/tests/integration/` y `langgraph-python/tests/integration/` (o la convención equivalente del repositorio).
- Composición de dependencias en `adk-go/internal/app/app.go` y `langgraph-python/app/main.py` para seleccionar el LLM controlado sin alterar el modo normal.
- Interfaces/adaptadores LLM (`adk-go/internal/platform/openrouter/` y el adaptador LLM Python), contratos de resume y exposición del resultado/estado sólo si son necesarios para hacer verificable el flujo HTTP.
- Servicios de jobs y runtime (`adk-go/internal/jobs/`, `adk-go/internal/runtime/`, `langgraph-python/app/jobs/`, `langgraph-python/app/agent/`) únicamente para conservar el job ID, estado HITL y resume observables.
- Documentación de ejecución y, si se agregan variables, `.env.example` de cada runtime.

## Constraints

- No editar manualmente archivos generados por sqlc ni mover SQL de repositorios a handlers/agentes.
- PostgreSQL SHALL seguir siendo la fuente de persistencia; SQLAlchemy async en Python y pgx/sqlc en Go conservarán sus convenciones.
- Redis y RabbitMQ SHALL ser los clientes/adaptadores reales ya usados por cada runtime, no dobles de test en los escenarios E2E.
- No introducir autenticación ni inventar un endpoint público nuevo sólo para facilitar asserts; si el resume no es observable con el contrato actual, se deberá documentar y resolver mediante el contrato HTTP existente antes de implementar la suite.
- Los Compose de Go y Python SHALL seguir siendo independientes y cada uno deberá poder levantarse sin el otro.

## Edge Cases

- Contenedor arrancado pero servicio aún no listo; reinicio del worker durante polling; cola con mensaje residual; volumen previo no limpio.
- Usuario A solicitando la reserva de B, dos jobs con el mismo thread/session, job duplicado y resume sin checkpoint.
- Confirmación con `confirmed=false`, selección faltante, selección no ofrecida, `interrupt_id`/name incorrectos y resume repetido.
- LLM local caído, respuesta HTTP no-2xx, JSON no válido, estructura desconocida, respuesta inesperada y más de una respuesta para una llamada.
- Inventario insuficiente, crédito expirado/insuficiente, tarifa no modificable, vuelo cancelado y error posterior a una escritura; todos SHALL demostrar rollback o ausencia de mutación.
- Ejecución concurrente sobre el último asiento o el mismo crédito: como mínimo una operación debe fallar sin sobreventa ni doble consumo.

## Error Handling

El harness SHALL marcar la prueba como fallida si no puede conectar a un servicio, si vence el polling o si el estado final no coincide. Los fallos de negocio deberán distinguirse de fallos de infraestructura. Tras un error de procesamiento se deberá comprobar Redis/RabbitMQ y PostgreSQL antes del cleanup; cualquier ack de éxito con estado o mutación incompleta será un defecto. Los errores del fake LLM deberán propagarse por el mismo camino que un error del proveedor, sin convertirlos en confirmación ni en éxito.

## Acceptance Criteria

- Se puede ejecutar cada suite desde su runtime con un comando documentado, sin OpenRouter, API keys ni red externa.
- PostgreSQL, Redis y RabbitMQ se prueban como servicios reales y el test falla si alguno se reemplaza por un mock o no está listo.
- El flujo HTTP de al menos un usuario llega al agente real a través de RabbitMQ y su estado Redis termina correctamente.
- Existe un escenario de pausa/resume confirmado que no modifica la base antes de confirmar y que verifica en PostgreSQL la mutación completa después.
- Existe un escenario declinado/inválido/error que demuestra ausencia de mutación parcial.
- Hay datos para varios usuarios y un test comprueba aislamiento de reservas/créditos entre ellos.
- Go y Python pasan los mismos invariantes de persistencia: segmento, inventario, crédito, total e historial.
- Los tests son repetibles desde un estado limpio y dejan diagnóstico útil cuando fallan.
- `go test ./...` y los checks Python existentes siguen pasando; el comando de integración queda separado y documentado para no hacer que tests unitarios dependan de Docker.

## Test Scenarios

1. **Confirmado con crédito:** Alice inicia cambio de ABC123, el fake LLM selecciona AS101/ECO, el job espera confirmación, resume confirmado y se verifican inventario, crédito, total, segmento e historial.
2. **Confirmado sin crédito:** Bruno cambia DEF456 a una opción cuyo importe no requiere crédito; se verifica que no se consume crédito y que el total/reglas son correctos.
3. **Aislamiento:** Carla no puede consultar ni mutar la reserva de Alice usando su propia identidad; no se crea `flight_changes`.
4. **Declinado:** se alcanza la pausa, se reanuda con `confirmed=false` y todas las filas mutables conservan el snapshot inicial.
5. **Retry controlado:** la primera evaluación produce una selección inválida y la segunda una válida; se verifica la ruta y que no hubo mutación en el intento inválido.
6. **Fallo LLM:** el fake devuelve JSON inválido/no-2xx; el job no termina en éxito, el mensaje no se confirma como éxito y PostgreSQL permanece igual.
7. **Resume inválido:** faltan selección, interrupt o thread/checkpoint; el runtime rechaza o falla de forma determinista sin mutar.
8. **Rollback real:** se prepara inventario/crédito insuficiente o un fallo de escritura reproducible; no quedan cambios parciales ni doble consumo.
9. **Concurrencia:** dos jobs independientes compiten por el último asiento/crédito; el resultado agregado no sobrevende ni consume dos veces.
10. **Repetibilidad/cleanup:** se ejecuta la suite dos veces con volúmenes nuevos y se obtienen los mismos resultados y conteos.

## Out of Scope

- Llamadas a OpenRouter u otro LLM comercial.
- Benchmark, pruebas de carga o medición comparativa de latencia/CPU.
- Sustituir la suite unitaria existente o probar cada repositorio con mocks.
- Autenticación/autorización real y cambios de modelo de identidad más allá de simular `user_id`/thread/session.
- Persistir checkpoints nuevos fuera del mecanismo ya utilizado por cada runtime.
- Compartir una única infraestructura entre los Compose de Go y Python.

## Assumptions

- El contrato actual de chat/resume se conservará: Go usa `user_id`/`session_id` y su objeto `resume`; Python usa `thread_id` y `input`/`resume`.
- El schema y las reglas de rebooking existentes son la fuente de verdad; los fixtures deberán adaptarse a cualquier diferencia actual entre los dos schemas sin cambiar su semántica.
- Es aceptable añadir un adaptador/fake LLM de integración local y una opción explícita de composición para test, siempre que no cambie el comportamiento por defecto.
- La observación de la pausa Go puede requerir instrumentar el contrato existente para devolver/registrar `interrupt_id` y `name`; si no puede hacerse mediante HTTP sin exponer información nueva, esa limitación deberá resolverse explícitamente y no ocultarse en el harness.

## Ambiguities to Resolve Before Implementation

1. El endpoint Python acepta jobs pero no expone actualmente una respuesta de resultado ni un endpoint de consulta; debe confirmarse si Redis es el contrato de lectura del estado y cómo se obtienen los datos de resume por HTTP.
2. El endpoint Go publica `resume`, pero las sesiones ADK son en memoria y se pierden al reiniciar; la suite deberá probar resume sin reiniciar el proceso o se deberá acordar persistencia distinta.
3. `langgraph-python/app/main.py` importa un adaptador `app.platform.llm` que no aparece entre los archivos inspeccionados; antes de implementar se debe confirmar su estado y su interfaz inyectable.
4. Go no monta schema/fixtures en Compose y Python monta sólo schema, no `mock_data.sql`; debe decidirse si el harness usa un contenedor/init separado o un paso explícito de `psql`, manteniendo ambos stacks reproducibles.
