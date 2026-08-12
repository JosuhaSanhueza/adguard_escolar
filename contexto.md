# Contexto de Proyecto: AdGuard Home Custom Fork (GameControl & Visualizations)

## 1. Visión General del Proyecto
Este proyecto es un fork personalizado de **AdGuard Home** (Backend en Go, Frontend en React/TypeScript). 
El objetivo es adaptar la interfaz y la lógica de filtrado para entornos educativos (laboratorios de computación), permitiendo control granular del acceso a juegos por IP/PC y vinculando las métricas del dashboard a listas de bloqueo específicas.

---

## 2. Pila Tecnológica (Tech Stack)
- **Backend:** Go (Golang) — Motor DNS, servidor HTTP API, parsing de reglas y gestión de clientes/configuración (`AdGuardHome.yaml`).
- **Frontend:** React, TypeScript, Redux/Context (según versión de AGH), TailwindCSS / CSS Modules.
- **Rutas de API REST:** Endpoints personalizados bajo `/control/` para la gestión de GameControl.

---

## 3. Requerimientos de Modificación

### Requerimiento A: Personalización de Tarjetas de Estadísticas (Dashboard)
1. **Tarjeta de Bloqueos de Malware:**
   - Debe filtrar y contar únicamente las peticiones DNS bloqueadas que coincidan con los IDs o dominios de las **listas de seguridad Hagezi** instaladas.
2. **Tarjeta de Contenido Adulto:**
   - Debe contabilizar de forma dedicada las peticiones bloqueadas pertenecientes a la lista **OISD NSFW**.
## listas de bloqueo

HaGeZi's Normal Blocklist https://adguardteam.github.io/HostlistsRegistry/assets/filter_34.txt
HaGeZi's Encrypted DNS/VPN/TOR/Proxy Bypass https://adguardteam.github.io/HostlistsRegistry/assets/filter_52.txt
HaGeZi's Anti-Piracy Blocklist https://adguardteam.github.io/HostlistsRegistry/assets/filter_46.txt
HaGeZi's DynDNS Blocklist https://adguardteam.github.io/HostlistsRegistry/assets/filter_54.txt
HaGeZi's Threat Intelligence Feeds https://adguardteam.github.io/HostlistsRegistry/assets/filter_44.txt
HaGeZi's Badware Hoster Blocklist https://adguardteam.github.io/HostlistsRegistry/assets/filter_55.txt
OISD NSFW https://nsfw.oisd.nl/
Games https://raw.githubusercontent.com/JosuhaSanhueza/BlockList/refs/heads/main/GamesBlockList.txt

---

### Requerimiento B: Nuevo Módulo "GameControl"
Ubicación en UI: Menú lateral / superior -> **Filtros (Filters) -> GameControl** (`/filters/gamecontrol`).

#### 1. Fuente de Lista de Bloqueo (Upstream List):
- Carga de reglas desde una URL raw de GitHub que contiene dominios de juegos (ej. lista `games` de GitHub).
- El backend debe sincronizar/cachear esta lista periódicamente.

#### 2. Gestión Modular de Segmento / Rango de IPs:
- Rango por defecto configurable desde la UI: `192.168.12.101` al `192.168.12.145` (45 equipos por defecto, escalable).
- Campo de búsqueda y filtrado rápido por IP, Nombre de Host (`PC1`, `PC2`, etc.) o estado.

#### 3. Control de Bloqueo Granular y Global:
- **Botón Maestro ("Bloquear Todo / Desbloquear Todo"):** Aplica o remueve la regla de bloqueo de juegos a todas las IPs del rango definido simultáneamente.
- **Controles Granulares por Host:** Cada equipo (`PC1`, `PC2`... `PCN`) debe tener su propio switch/toggle independiente.
- **Casos de uso:** El profesor desbloquea el juego únicamente a los alumnos/PCs que terminaron sus tareas de laboratorio.

#### 4. Lógica Interna de Aplicación (Backend):
- Vincular la lista de juegos a reglas de cliente de AdGuard Home (`$client='192.168.12.101'`), o gestionar un mapeo dinámico de DNS blocking en tiempo real según el estado activo en la interfaz.

---

## 4. Estructura Esperada de Archivos a Modificar / Crear

- `client/src/components/Filters/GameControl/` *(Nuevo componente React para la UI de GameControl)*
- `client/src/components/Dashboard/` *(Ajustes en el conteo de las tarjetas de malware y NSFW)*
- `internal/home/` *(Nuevos handlers HTTP y structs para gestionar la configuración de GameControl)*
- `internal/filtering/` *(Ajuste en la lógica de resolución DNS para aplicar reglas por IP)*

---

## 5. Instrucciones para la IA
1. Proponer cambios respetando las buenas prácticas de Go y React.
2. Mantener la compatibilidad con el archivo de configuración `AdGuardHome.yaml`.
3. Mantener una interfaz limpia e integrada con el diseño nativo de AdGuard Home.
