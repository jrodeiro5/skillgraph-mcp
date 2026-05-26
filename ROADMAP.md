# ROADMAP: Integración de Grafo de Capacidades en `skillgraph-mcp`

Este documento detalla las tareas pendientes para implementar un modelo de grafo de capacidades en el fork `skillgraph-mcp` y mejorar el descubrimiento dinámico de herramientas y la orquestación.

## Fase 1: Estructura de Datos del Grafo (Go) [Completado]
- [x] Crear el paquete `internal/graph`.
- [x] Definir los tipos de nodo: `Skill`, `Tool`, `Resource`.
- [x] Definir los tipos de relación: `HAS_TOOL`, `PREREQUISITE_FOR`, `PRODUCES`, `REQUIRES`, `COMMON_NEXT_STEP`.
- [x] Implementar la estructura en memoria `Graph` (CRUD de nodos y aristas).
- [x] Implementar un serializador compacto para LLMs (ej: formato Markdown / Lattice de relaciones).
- [x] Escribir pruebas unitarias en `internal/graph/graph_test.go`.

## Fase 2: Inferencia de Relaciones y Configuración [Completado]
- [x] Extender la configuración en `internal/config/config.go` para permitir relaciones estáticas definidas por el usuario.
- [x] Integrar el grafo en `internal/mcpserver/manager.go`.
- [x] Diseñar el motor de inferencia semántica automática durante la carga:
  - Vincular automáticamente `Skill -[HAS_TOOL]-> Tool`.
  - Inferir relaciones `PRODUCES`/`REQUIRES` analizando y cruzando los schemas JSON de entrada y salida (coincidencia de tipos y nombres como `xxx_id`).
  - Agrupar herramientas que operen sobre entidades comunes por prefijo (ej: `github_create_issue` y `github_update_issue`).

## Fase 3: Nuevas Herramientas MCP para el Agente [Completado]
- [x] Implementar la herramienta `get_skill_graph`:
  - Permitir consultar el grafo completo o filtrado por una skill específica.
- [x] Implementar la herramienta `plan_workflow`:
  - Recibir un string descriptivo (meta de alto nivel) y devolver un camino recomendado de ejecución de herramientas.
- [x] Registrar estas herramientas en el servidor principal (`internal/app/server.go`).
- [x] Escribir pruebas e2e y de integración para las nuevas herramientas.

## Fase 4: Autogeneración Offline de Metadatos (Bootstrap) [Completado]
- [x] Crear un comando o script offline (`cmd/bootstrap_metadata` / CLI flag) que analice los servidores MCP configurados.
- [x] Usar un modelo/API para generar descripciones claras y orientadas a casos de uso de cada herramienta y skill (automatización similar a Adala).
- [x] Escribir estas descripciones actualizadas en la configuración para optimizar el enrutamiento inicial.

## Fase 5: Lattice Semántico en Markdown [Completado]
- [x] Crear la carpeta `.mcp_lattice`
- [x] Agregar un generador en Go para crear `skills.md` y `relations.md`
- [x] Exponer la herramienta `read_lattice` (`internal/tools/read_lattice.go`)

## Fase 6: Descargador de Documentación Nativo (go-github) [Completado]
- [x] Agregar `github.com/google/go-github/v60` a `go.mod`
- [x] Implementar el descargador de READMEs en `internal/docs/fetcher.go`
- [x] Vincular la descarga de documentación en el inicio de los skills
- [x] Verificar el correcto funcionamiento de la descarga y tests

## Fase 7: Optimización Dinámica de Habilidades (SkillOpt) [Completado]
- [x] Implementar logging de trayectorias (rollouts) en `execute_code.go` usando `TraceCollector` context-based.
- [x] Guardar trazas como JSON en `.mcp_lattice/traces/`.
- [x] Crear un demonio en segundo plano (`startOptimizationLoop` en `refine/engine.go`) que analice trazas acumuladas periódicamente.
- [x] Implementar bucle de optimización de texto (SkillOpt) con prompts para DeepSeek/Gemini que sugieran modificaciones y relaciones en `mcp.json`.
- [x] Validar que las propuestas no contengan nodos alucinados antes de persistir los cambios en la configuración.
- [x] Escribir pruebas unitarias correspondientes en `execute_code_test.go` y `engine_test.go` para verificar el correcto funcionamiento.

