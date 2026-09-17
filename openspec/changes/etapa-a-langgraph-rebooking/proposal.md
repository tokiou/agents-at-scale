# Etapa A: workflow de rebooking en LangGraph/Python

## Objective

Construir el esqueleto ejecutable del workflow de rebooking del agente LangGraph/Python, manteniendo la semántica del workflow equivalente de `adk-go`: estado compartido tipado, nodos asíncronos con dependencias inyectables, búsqueda paralela de contexto, unión de resultados, pausa para confirmación humana y rutas condicionales. La etapa debe poder probarse completamente con dobles, sin depender de persistencia, Redis/RabbitMQ operativos ni de una operación PostgreSQL que efectúe el rebooking.

## Relevant Context

- El runtime Python vive en `langgraph-python/app`; el módulo de dominio de airline vive en `app/airline/`.
- `app/agent/state.py`, `app/agent/graph.py` y `app/agent/prompts.py` son actualmente stubs, y `app/agent/nodes/` sólo contiene `.gitkeep`.
- `app/airline/service.py` ya expone `get_reservation`, `get_available_travel_credits` y `search_rebooking_options`, con validaciones de ventana y cantidad de pasajeros.
- Las respuestas de dominio se modelan con Pydantic en `app/airline/schemas.py`; los repositorios se inyectan en `app/airline/service.py`.
- El workflow Go existente en `adk-go/internal/agent/` es la referencia de comportamiento: `understand_request -> get_reservation -> (search_alternatives || get_travel_credits) -> evaluate_options -> ask_confirmation -> validate_change`, con invalidación y reintento.
- El `Makefile` Python ejecuta `compileall` y `unittest`; los tests existentes usan `unittest.IsolatedAsyncioTestCase` y fakes asíncronos.

## Scope

Incluye los contratos, nodos y construcción del grafo de la Etapa A bajo `langgraph-python/app/agent/`, más tests unitarios del workflow.

No incluye endpoints HTTP, consumo/publicación de jobs, almacenamiento de checkpoints, sesiones persistentes, integración real con Redis/RabbitMQ/PostgreSQL, ni la mutación de una reserva para ejecutar o verificar un rebooking.

## Expected Behavior

El grafo debe representar la siguiente secuencia:

```text
START -> understand_request -> get_reservation
                              |-- search_alternatives --|
                              |-- get_travel_credits --| -> evaluate_options
                                                        -> ask_confirmation (interrupt)
                                                        -> validate_change
                                     valid -> final result (ready)
                                  invalid -> explain_invalid_change -> retry -> evaluate_options
                                declined -> final result (declined)
```

`search_alternatives` y `get_travel_credits` deben ejecutarse como fan-out desde el contexto de reserva y `evaluate_options` sólo puede ejecutarse después del fan-in de ambos resultados. Un `interrupt` debe detener la ejecución antes de validar la selección; al reanudar, el estado previo y la evaluación deben conservarse.

## Functional Requirements

### Estado compartido

1. El módulo `app.agent.state` SHALL definir un estado compartido explícito y tipado para todo el workflow; los nodos SHALL recibir el estado y devolver únicamente actualizaciones de sus campos.
2. El estado SHALL distinguir, como mínimo, entrada/mensaje, `RebookingRequest`, contexto de reserva, resultado de alternativas, créditos disponibles, evaluación, selección/decisión de confirmación, resultado de validación, ruta/estado final y motivo de error o reintento.
3. El estado SHALL conservar el resultado de cada rama paralela con claves estables y no SHALL depender de variables globales, atributos mutables de módulo ni estado implícito del nodo.
4. Los contratos del workflow SHALL reutilizar los schemas de `app.airline.schemas` cuando representen datos del dominio y SHALL mantener UUID, `datetime`, `Decimal` y enums sin convertirlos arbitrariamente a strings.
5. El estado SHALL ser serializable por LangGraph para permitir un futuro checkpointer, pero la Etapa A SHALL usar únicamente memoria durante las pruebas y no SHALL requerir persistirlo.

### Dependencias y nodos

6. `app.agent.nodes` SHALL exponer nodos o fábricas de nodos para, como mínimo: `understand_request`, `get_reservation`, `search_alternatives`, `get_travel_credits`, `evaluate_options`, `ask_confirmation`, `validate_change` y `explain_invalid_change`.
7. Cada nodo que necesite LLM, servicio airline, reloj u otra colaboración SHALL recibirla mediante una dependencia/fachada/protocolo inyectado. Los nodos no SHALL crear clientes, repositorios, engines, Redis, RabbitMQ ni servicios concretos.
8. Las dependencias SHALL poder sustituirse por fakes o mocks asíncronos en tests. El nodo de rebooking de esta etapa, si se deja como punto de extensión, SHALL ser un puerto inyectable y no una llamada obligatoria a PostgreSQL.
9. Los nodos SHALL validar la presencia y coherencia de sus entradas antes de invocar dependencias; los errores de dependencia SHALL propagarse como errores del nodo sin ocultarse como una confirmación válida.
10. `understand_request` SHALL producir una solicitud estructurada; `get_reservation` SHALL cargar el contexto asociado al booking reference; las ramas de búsqueda SHALL delegar las reglas existentes a `app.airline.service.Service` o a su doble equivalente.
11. `evaluate_options` SHALL recibir ambos resultados del fan-in, producir una evaluación estructurada y no SHALL inventar IDs de opciones que no estén en las alternativas recibidas.

### Grafo y rutas

12. `app.agent.graph` SHALL exponer una fábrica que construya/compile el grafo sin fijar dependencias reales en tiempo de importación.
13. El grafo SHALL contener fan-out desde `get_reservation` hacia `search_alternatives` y `get_travel_credits`, y fan-in explícito antes de `evaluate_options`; una evaluación no SHALL ejecutarse con sólo una de las ramas completada.
14. `ask_confirmation` SHALL invocar el mecanismo `interrupt` de LangGraph con la evaluación y las opciones ofrecidas como payload, y SHALL aceptar una reanudación mediante el mecanismo de `Command`/resume compatible con la versión declarada de `langgraph`.
15. La pausa de confirmación SHALL ocurrir antes de cualquier validación o mutación de rebooking. Reanudar con una selección/decisión SHALL hacer que `validate_change` use la evaluación conservada, no una evaluación reconstruida de forma implícita.
16. `validate_change` SHALL producir una ruta `valid` cuando la decisión está confirmada y la selección pertenece a las opciones ofrecidas; SHALL producir `invalid` cuando la selección no está ofrecida o el doble de la validación de dominio la rechaza; y SHALL producir `declined` cuando el usuario no confirma.
17. La ruta `invalid` SHALL conservar la razón y pasar por `explain_invalid_change`. `explain_invalid_change` SHALL emitir un resultado explicable y una señal `retry` para volver a `evaluate_options` sin perder la solicitud, las alternativas ni los créditos; la implementación SHALL evitar un bucle infinito silencioso y SHALL conservar/incrementar un contador de reintentos.
18. La ruta `declined` SHALL ser terminal para esta ejecución y SHALL emitir un resultado que indique que no se confirmó el cambio; no SHALL invocar una operación de rebooking.
19. La ruta `valid` SHALL ser terminal `ready` (o equivalente explícito) y SHALL exponer selección y validación necesarias para una futura ejecución; en Etapa A no SHALL ejecutar ni verificar la mutación PostgreSQL.
20. Los nombres de nodos y las transiciones SHALL ser inspeccionables en tests, de forma que pueda verificarse el fan-out/fan-in y cada una de las rutas `valid`, `invalid`, `retry` y `declined`.

### Tests

21. Los tests SHALL estar en `langgraph-python/tests/`, seguir el patrón `unittest` existente y usar fakes/mocks para LLM, airline service y confirmación; no SHALL necesitar contenedores ni credenciales.
22. SHALL existir un test de estado inicial y de actualización aislada de nodos, incluyendo que un nodo no borra campos producidos por otro nodo.
23. SHALL existir un test que verifique que ambas ramas paralelas reciben el mismo contexto, que ambas se completan antes de evaluar y que sus resultados llegan juntos al fan-in.
24. SHALL existir un test del interrupt que compruebe que la primera invocación queda pausada, que no llama validación/rebooking y que una reanudación con confirmación continúa con el mismo estado evaluado.
25. SHALL existir al menos un escenario de grafo para cada ruta: selección válida (`valid`), selección no ofrecida o rechazada (`invalid`), nuevo ciclo (`retry`) y usuario no confirmado (`declined`).
26. SHALL existir tests de errores para entrada incompleta/mal formada y para excepciones de una dependencia; deben comprobar que no se reporta éxito ni se ejecuta un rebooking.
27. Los tests SHALL poder ejecutarse con `langgraph-python/Makefile` y con `python -m unittest discover -s tests -p 'test_*.py'` desde `langgraph-python`.

## Non-Functional Requirements

- El diseño SHALL ser async-compatible con el servicio Python existente y SHALL usar anotaciones de tipo en contratos públicos.
- La construcción del grafo SHALL ser determinista y no SHALL abrir conexiones externas ni producir efectos laterales al importar `app.agent`.
- Las claves de estado y rutas SHALL ser estables para facilitar comparación con el workflow Go y futura observabilidad.

## Affected Components

- `langgraph-python/app/agent/state.py`: contratos del estado y resultados del workflow.
- `langgraph-python/app/agent/nodes/` y/o módulos del mismo paquete: fábricas e implementación de nodos con dependencias inyectadas.
- `langgraph-python/app/agent/graph.py`: construcción, fan-out/fan-in, interrupt y routing.
- `langgraph-python/app/agent/prompts.py`: prompts estructurados sólo si son necesarios para los nodos LLM.
- `langgraph-python/tests/`: tests unitarios del grafo y nodos con dobles.

## Constraints

- Debe respetarse la arquitectura Python del repositorio: reglas en `app/airline/service.py`, adaptadores bajo `app/platform` y composición separada de dominio.
- No se deben añadir SQL crudo de airline ni exponer clientes de base de datos a nodos.
- La dependencia `langgraph>=0.3,<1.0` es la interfaz de ejecución a considerar; los tests deben usar la API de interrupt/resume compatible con la versión instalada.
- No se debe modificar código de producción fuera de los componentes del agente requeridos por esta especificación, ni modificar los tests existentes de servicio salvo que sea imprescindible para compatibilidad.

## Edge Cases

- Solicitud sin booking reference, segmento ambiguo o ventana de fechas ausente/invertida.
- Reserva inexistente, sin segmentos o con más de un segmento sin selección explícita.
- Cero alternativas, cero créditos o tarifa original no cambiable: la evaluación debe ser válida como resultado de negocio, no una confirmación automática.
- Selección con IDs desconocidos, selección perteneciente a una evaluación anterior o payload de resume con forma inválida.
- Reanudar con rechazo, repetir un `retry` y alcanzar el límite de reintentos definido por el contrato del estado.
- Fallo de una sola rama paralela o del LLM: el grafo no debe entrar al fan-in como si la rama hubiese tenido éxito.

## Error Handling

Los errores de validación de entrada y de dependencias SHALL quedar diferenciados de las decisiones de negocio (`invalid`/`declined`). Un error técnico SHALL finalizar la ejecución como fallida y conservar un mensaje seguro y el nodo que falló; no SHALL convertirse en `valid`, `declined` ni en un `ack` exitoso de jobs. Una selección inválida SHALL conservar su razón, permitir el `retry` especificado y no SHALL mutar datos persistentes.

## Acceptance Criteria

- Existe un grafo LangGraph compilable cuya estructura permite observar `START`, los nodos nombrados, el fan-out/fan-in y las rutas solicitadas.
- El estado tipado contiene explícitamente los datos de entrada, ambas salidas paralelas, evaluación, confirmación, validación, ruta final y reintentos.
- El interrupt se activa antes de validar y una reanudación confirmada alcanza `valid` sin reconstruir ni perder la evaluación.
- Los cuatro escenarios de routing (`valid`, `invalid`, `retry`, `declined`) están cubiertos por tests deterministas con dobles.
- Ningún test o ejecución de Etapa A requiere PostgreSQL, Redis, RabbitMQ, checkpointer persistente o una operación real de rebooking.
- `make test` en `langgraph-python` continúa pasando junto con los tests ya existentes de `app.airline.service`.

## Test Scenarios

1. Request válido con reserva, dos ramas exitosas y una opción ofrecida: pausa, resume confirmado, resultado terminal `ready/valid`.
2. Resume con `confirmed=false`: resultado terminal `declined`, sin llamada de validación ni ejecución.
3. Resume con opción no ofrecida: resultado `invalid` con razón y transición `retry`; segundo ciclo con nueva opción válida.
4. Servicio de validación que devuelve una regla de negocio inválida: misma ruta `invalid/retry`, sin mutación.
5. Sin alternativas: fan-in completo, evaluación sin opciones y comportamiento explícito que no confirma automáticamente.
6. Error en búsqueda de alternativas o créditos: error de workflow, sin evaluación parcial ni éxito.
7. Entrada incompleta y resume mal formado: error verificable, sin llamadas posteriores.
8. Reintentos hasta el límite del estado: finalización controlada, sin bucle infinito.

## Out of Scope

- Checkpointing, persistencia de estado de conversación o resultados.
- Endpoints FastAPI para iniciar/reanudar el agente.
- Llamadas reales a PostgreSQL para `execute_rebooking` o `verify_rebooking`, transacciones, idempotencia de rebooking y reserva de inventario.
- Integración real con LLM, herramientas externas, Redis, RabbitMQ o telemetría de producción.
- Cambios equivalentes en `adk-go`; se usa su workflow sólo como referencia de paridad.

## Assumptions

- La semántica acordada de Etapa A es la del workflow Go actualmente presente, adaptada a APIs idiomáticas de LangGraph/Python.
- `valid` significa "selección validada y lista para una futura ejecución", no "rebooking ya aplicado".
- Las opciones y créditos pueden representarse con los schemas existentes y los dobles pueden devolver objetos compatibles con esos schemas.
- El límite exacto de reintentos será un parámetro explícito del estado/dependencias; si el producto no define un valor, la implementación deberá documentarlo en el contrato y en los tests, sin permitir loops ilimitados.
