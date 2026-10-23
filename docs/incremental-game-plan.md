# Plan de Desarrollo: Juego Incremental (Clicker) en Roblox Studio

Este documento detalla la arquitectura, componentes y orden de ejecucion para construir un juego incremental completo, testeable y funcional en Roblox Studio mediante nuestro servidor MCP.

---

## 1. Resumen del Juego

* **Objetivo principal**: Los jugadores hacen clic en una moneda dorada 3D o en un boton central en pantalla para acumular "Clicks".
* **Economia**:
  * **Clicks**: Moneda base del juego.
  * **Poder de Click**: Cantidad de clicks que otorga cada pulsacion (inicia en 1).
  * **Auto-Clickers**: Cantidad de clicks pasivos que se suman automaticamente por segundo (inicia en 0).
  * **Rebirths**: Reinicio de progreso a cambio de un multiplicador permanente (x2, x3, etc.).
* **Tienda**: Menu desplegable con mejoras acumulativas y aumento progresivo de precios.
* **Feedback visual**: Moneda flotante con rotacion continua, particulas doradas, sonidos de impacto y numeros flotantes (+1, +2) animados con TweenService.

---

## 2. Mapa de Instancias y Arquitectura

```
game
├── Workspace
│   └── ClickAltar (Model)
│       ├── Pedestal (Part, base de marmol/pizarra)
│       ├── CoinPart (Part, cilindro de oro brillante con rotacion)
│       │   ├── ClickDetector (Permite hacer clic directo en el mundo 3D)
│       │   ├── Sparkles / ParticleEmitter (Efecto de chispas)
│       │   ├── PointLight (Iluminacion calida sobre la moneda)
│       │   └── ClickSound (Sonido agradable al hacer click)
│       └── AltarLabel (BillboardGui, texto flotante "Haz clic aqui")
│
├── ReplicatedStorage
│   ├── ClickRemote (RemoteEvent, peticion de click desde el cliente)
│   ├── BuyUpgradeRemote (RemoteFunction, peticion y validacion de compra)
│   └── UpgradeConfig (ModuleScript, precios base y multiplicadores)
│
├── ServerScriptService
│   ├── LeaderstatsManager (Script de servidor)
│   │   ├── Crea la carpeta leaderstats (Clicks, Multiplicador, Rebirths)
│   │   ├── Procesa clicks y aplica bonus de Rebirths
│   │   └── Bucle de auto-clicker cada segundo
│   └── UpgradeManager (Script de servidor)
│       └── Valida compras, descuenta saldo y escala precios
│
└── StarterGui
    └── ClickerHUD (ScreenGui)
        ├── ClickButton (Boton central inferior para clics rapidos)
        ├── StatsDisplay (Panel superior mostrando Clicks y tasa por segundo)
        ├── ShopToggleButton (Boton lateral para abrir/cerrar la tienda)
        ├── ShopModal (Ventana de mejoras con UICorner y UIStroke)
        │   ├── TitleBar (Titulo y boton de cierre X)
        │   └── UpgradesList (ScrollingFrame con UIListLayout)
        │       ├── ItemCard_ClickPower (Mejora de clic)
        │       ├── ItemCard_AutoClicker (Mejora de auto-clic)
        │       └── ItemCard_Rebirth (Boton de renacimiento)
        └── HUDController (LocalScript para animaciones, sonidos y remotos)
```

---

## 3. Fases de Construccion

### Fase 1: Construccion 3D del Altar en Workspace
1. Crear el modelo `ClickAltar` en las coordenadas centrales `(0, 1, 0)`.
2. Modelar el `Pedestal` de apoyo (cilindro o bloque oscuro pulido).
3. Colocar la `CoinPart` dorada flotante con material `Neon` o `Metal`, `Color3.fromRGB(255, 215, 0)` y tamaño proporcional.
4. Agregar el `ClickDetector` con rango de activacion de 32 studs.
5. Agregar `ParticleEmitter` con textura de destellos y `PointLight` dorada.
6. Agregar script local o de animacion suave para que la moneda gire lentamente sobre su eje Y.

### Fase 2: Sistema de Datos y Red en ReplicatedStorage
1. Crear el `RemoteEvent` nombrado `ClickRemote` para sincronizar los clics.
2. Crear el `RemoteFunction` nombrado `BuyUpgradeRemote` para responder al cliente con exito o saldo insuficiente.
3. Crear el `ModuleScript` `UpgradeConfig` con las formulas de coste:
   * Coste = CosteBase * (1.15 ^ NivelActual).

### Fase 3: Logica de Servidor en ServerScriptService
1. Escuchar el evento `Players.PlayerAdded` y generar la carpeta `leaderstats` con:
   * `Clicks` (IntValue)
   * `Multiplicador` (IntValue, base 1)
   * `AutoClicks` (IntValue, base 0)
   * `Rebirths` (IntValue, base 0)
2. Conectar `ClickRemote.OnServerEvent` para sumar `Multiplicador * (Rebirths + 1)` a los `Clicks` del jugador.
3. Conectar `BuyUpgradeRemote.OnServerInvoke` para procesar compras de forma segura en el servidor.
4. Iniciar un bucle de fondo que cada 1 segundo recorra a los jugadores conectados y les sume sus `AutoClicks`.

### Fase 4: Interfaz Grafica (HUD) en StarterGui
1. Crear el `ScreenGui` con `ResetOnSpawn = false`.
2. Diseñar el boton `ClickButton` con esquinas redondeadas (`UICorner`), borde (`UIStroke`) y efecto visual de escala al presionarlo.
3. Diseñar la ventana de la tienda con `ScrollingFrame` y tarjetas individuales para cada mejora.
4. Conectar los botones con el script cliente `HUDController` para abrir y cerrar el menu de forma interactiva.

### Fase 5: Efectos y Pulido Visual
1. Crear texto flotante (+X) animado con `TweenService` que suba y se desvanezca al hacer clic.
2. Añadir efectos de sonido al hacer clic y al comprar mejoras.
3. Verificar el bucle completo jugando en Studio.

---

## 4. Criterios de Exito
* El jugador puede hacer clic tanto en la moneda 3D como en el boton de la pantalla.
* Los valores de la tabla de clasificacion (arriba a la derecha) se actualizan en tiempo real.
* La tienda no permite comprar si no hay suficientes clicks.
* El auto-clicker suma clicks sin intervencion manual.
* Todo se ejecuta de forma segura en el servidor sin posibilidad de manipulacion desde el cliente.
