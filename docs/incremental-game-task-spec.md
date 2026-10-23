# Especificacion de Tarea para Agente: Implementacion de Juego Incremental

Este archivo contiene la guia paso a paso, codigo Luau completo y cargas utiles estructuradas para que un agente de IA construya el juego incremental completo en Roblox Studio utilizando el servidor MCP `roblox-studio`.

---

## 1. Estado del Entorno

* **Servidor MCP**: Registrado en `mcp_config.json` como `roblox-studio` ejecutando `dist/roblox-studio-mcp.exe`.
* **Bridge HTTP**: Escuchando en `http://127.0.0.1:58741`.
* **Plugin de Studio**: Instalado, emparejado y conectado en modo edicion (`edit`).
* **Metodo de ejecucion**: El agente puede utilizar directamente las herramientas MCP (`build_tree`, `manage_script`, `execute_luau`, `set_properties`, etc.) o despachar codigo Luau a traves del puente.

---

## 2. Tarea 1: Construccion del Altar 3D en Workspace

### Objetivo
Crear en `game.Workspace` el modelo `ClickAltar` con pedestal, moneda dorada interactiva, detector de clicks, particulas y giro continuo.

### Estructura de Instancias
* `Workspace.ClickAltar` (Model)
  * `Pedestal` (Part): Cilindro o bloque oscuro pulido fijado en la base (`Anchored = true`, `CFrame = CFrame.new(0, 0.5, 0)`, `Size = Vector3.new(6, 1, 6)`, `Material = Slate`, `Color = Color3.fromRGB(40, 40, 45)`).
  * `CoinPart` (Part): Cilindro dorado flotante (`Anchored = true`, `CanCollide = false`, `CFrame = CFrame.new(0, 3.5, 0) * CFrame.Angles(0, 0, math.rad(90))`, `Size = Vector3.new(0.6, 3, 3)`, `Material = Metal`, `Color = Color3.fromRGB(255, 215, 0)`).
  * `CoinPart.ClickDetector` (ClickDetector): `MaxActivationDistance = 32`.
  * `CoinPart.PointLight` (PointLight): `Color = Color3.fromRGB(255, 220, 100)`, `Brightness = 2`, `Range = 12`.
  * `CoinPart.Sparkles` (Sparkles): `SparkleColor = Color3.fromRGB(255, 230, 120)`.
  * `CoinPart.SpinScript` (Script): Script de rotacion suave continua:
    ```lua
    local part = script.Parent
    local RunService = game:GetService("RunService")
    RunService.Heartbeat:Connect(function(dt)
        part.CFrame = part.CFrame * CFrame.Angles(0, dt * 1.5, 0)
    end)
    ```

---

## 3. Tarea 2: Red y Configuracion en ReplicatedStorage

### Objetivo
Configurar los canales de comunicacion cliente-servidor y el modulo con las estadisticas y formulas de precios.

### Instancias a crear
1. `ReplicatedStorage.ClickRemote` (RemoteEvent):
   * Maneja las peticiones de click provenientes de la moneda 3D o del boton de pantalla.
2. `ReplicatedStorage.BuyUpgradeRemote` (RemoteFunction):
   * Maneja las compras de mejoras y devuelve una tabla `{ success = true/false, message = "..." }`.
3. `ReplicatedStorage.UpgradeConfig` (ModuleScript):
   * Codigo fuente:
   ```lua
   local UpgradeConfig = {}

   UpgradeConfig.Upgrades = {
       ClickPower = {
           Name = "+1 Poder de Click",
           Description = "Aumenta la cantidad de clicks que ganas por cada pulsacion.",
           BaseCost = 15,
           CostMultiplier = 1.25,
           BaseBuff = 1,
       },
       AutoClicker = {
           Name = "+1 Auto-Clicker",
           Description = "Genera clicks pasivos automaticamente cada segundo.",
           BaseCost = 50,
           CostMultiplier = 1.30,
           BaseBuff = 1,
       },
       Rebirth = {
           Name = "Rebirth (Renacer)",
           Description = "Reinicia tus clicks y mejoras a cambio de un multiplicador x2 permanente.",
           BaseCost = 1000,
           CostMultiplier = 3.0,
           BaseBuff = 1,
       }
   }

   function UpgradeConfig.GetCost(upgradeId, currentLevel)
       local upg = UpgradeConfig.Upgrades[upgradeId]
       if not upg then return 0 end
       return math.floor(upg.BaseCost * (upg.CostMultiplier ^ currentLevel))
   end

   return UpgradeConfig
   ```

---

## 4. Tarea 3: Logica de Servidor en ServerScriptService

### Script 1: `ServerScriptService.LeaderstatsManager`
Maneja la conexion de jugadores, crea sus valores de clasificacion, conecta el detector de clicks del mundo 3D y ejecuta el bucle de auto-clicker.

```lua
local Players = game:GetService("Players")
local ReplicatedStorage = game:GetService("ReplicatedStorage")
local Workspace = game:GetService("Workspace")

local ClickRemote = ReplicatedStorage:WaitForChild("ClickRemote")

-- Inicializar estadisticas por jugador
Players.PlayerAdded:Connect(function(player)
    local leaderstats = Instance.new("Folder")
    leaderstats.Name = "leaderstats"
    leaderstats.Parent = player

    local clicks = Instance.new("IntValue")
    clicks.Name = "Clicks"
    clicks.Value = 0
    clicks.Parent = leaderstats

    local rebirths = Instance.new("IntValue")
    rebirths.Name = "Rebirths"
    rebirths.Value = 0
    rebirths.Parent = leaderstats

    -- Carpeta para datos tecnicos / mejoras
    local stats = Instance.new("Folder")
    stats.Name = "Stats"
    stats.Parent = player

    local clickPower = Instance.new("IntValue")
    clickPower.Name = "ClickPower"
    clickPower.Value = 1
    clickPower.Parent = stats

    local autoClicks = Instance.new("IntValue")
    autoClicks.Name = "AutoClicks"
    autoClicks.Value = 0
    autoClicks.Parent = stats
end)

-- Procesar clics
local function processClick(player)
    local leaderstats = player:FindFirstChild("leaderstats")
    local stats = player:FindFirstChild("Stats")
    if not (leaderstats and stats) then return end

    local clicks = leaderstats:FindFirstChild("Clicks")
    local rebirths = leaderstats:FindFirstChild("Rebirths")
    local clickPower = stats:FindFirstChild("ClickPower")

    if clicks and clickPower and rebirths then
        local gain = clickPower.Value * (rebirths.Value + 1)
        clicks.Value = clicks.Value + gain
    end
end

ClickRemote.OnServerEvent:Connect(processClick)

-- Conectar el ClickDetector de la moneda 3D si existe
task.spawn(function()
    local altar = Workspace:WaitForChild("ClickAltar", 10)
    if altar then
        local coin = altar:WaitForChild("CoinPart", 10)
        if coin then
            local detector = coin:WaitForChild("ClickDetector", 10)
            if detector then
                detector.MouseClick:Connect(processClick)
            end
        end
    end
end)

-- Bucle de Auto-Clicker (cada 1 segundo)
task.spawn(function()
    while true do
        task.wait(1)
        for _, player in ipairs(Players:GetPlayers()) do
            local leaderstats = player:FindFirstChild("leaderstats")
            local stats = player:FindFirstChild("Stats")
            if leaderstats and stats then
                local clicks = leaderstats:FindFirstChild("Clicks")
                local rebirths = leaderstats:FindFirstChild("Rebirths")
                local autoClicks = stats:FindFirstChild("AutoClicks")

                if clicks and autoClicks and rebirths and autoClicks.Value > 0 then
                    local gain = autoClicks.Value * (rebirths.Value + 1)
                    clicks.Value = clicks.Value + gain
                end
            end
        end
    end
end)
```

### Script 2: `ServerScriptService.UpgradeManager`
Procesa las compras solicitadas mediante `BuyUpgradeRemote`.

```lua
local ReplicatedStorage = game:GetService("ReplicatedStorage")
local UpgradeConfig = require(ReplicatedStorage:WaitForChild("UpgradeConfig"))
local BuyUpgradeRemote = ReplicatedStorage:WaitForChild("BuyUpgradeRemote")

BuyUpgradeRemote.OnServerInvoke = function(player, upgradeId)
    local leaderstats = player:FindFirstChild("leaderstats")
    local stats = player:FindFirstChild("Stats")
    if not (leaderstats and stats) then
        return { success = false, message = "Datos no inicializados" }
    end

    local clicks = leaderstats:FindFirstChild("Clicks")
    local rebirths = leaderstats:FindFirstChild("Rebirths")
    local clickPower = stats:FindFirstChild("ClickPower")
    local autoClicks = stats:FindFirstChild("AutoClicks")

    if upgradeId == "ClickPower" then
        local level = clickPower.Value - 1
        local cost = UpgradeConfig.GetCost("ClickPower", level)
        if clicks.Value >= cost then
            clicks.Value = clicks.Value - cost
            clickPower.Value = clickPower.Value + 1
            return { success = true, newLevel = clickPower.Value }
        else
            return { success = false, message = "Saldo insuficiente" }
        end

    elseif upgradeId == "AutoClicker" then
        local level = autoClicks.Value
        local cost = UpgradeConfig.GetCost("AutoClicker", level)
        if clicks.Value >= cost then
            clicks.Value = clicks.Value - cost
            autoClicks.Value = autoClicks.Value + 1
            return { success = true, newLevel = autoClicks.Value }
        else
            return { success = false, message = "Saldo insuficiente" }
        end

    elseif upgradeId == "Rebirth" then
        local cost = UpgradeConfig.GetCost("Rebirth", rebirths.Value)
        if clicks.Value >= cost then
            clicks.Value = 0
            clickPower.Value = 1
            autoClicks.Value = 0
            rebirths.Value = rebirths.Value + 1
            return { success = true, newLevel = rebirths.Value }
        else
            return { success = false, message = "Necesitas " .. cost .. " clicks para renacer" }
        end
    end

    return { success = false, message = "Mejora no valida" }
end
```

---

## 5. Tarea 4: Interfaz Grafica (HUD) en StarterGui

### Estructura del ScreenGui (`ClickerHUD`)
* `ResetOnSpawn = false`
* `ClickButton`: Boton central inferior (`Size = UDim2.new(0, 180, 0, 60)`, `Position = UDim2.new(0.5, -90, 0.88, 0)`, `BackgroundColor3 = Color3.fromRGB(0, 170, 255)`, `Text = "CLICK!"`, con `UICorner` y `UIStroke`).
* `ShopToggleBtn`: Boton lateral izquierdo (`Size = UDim2.new(0, 120, 0, 44)`, `Position = UDim2.new(0.04, 0, 0.88, 0)`, `BackgroundColor3 = Color3.fromRGB(80, 200, 120)`, `Text = "Tienda"`).
* `ShopFrame`: Ventana centrada (`Size = UDim2.new(0, 380, 0, 420)`, `Position = UDim2.new(0.5, -190, 0.5, -210)`, `Visible = false`, `BackgroundColor3 = Color3.fromRGB(30, 30, 35)`, con `UICorner`, `UIStroke`, barra de titulo y `CloseButton`).
* `ShopFrame.Scroll`: `ScrollingFrame` vertical con `UIListLayout` conteniendo las tarjetas de compra para `ClickPower`, `AutoClicker` y `Rebirth`.

### LocalScript: `ClickerHUD.HUDController`
Conecta los botones de la interfaz, envía clics a `ClickRemote`, invoca compras mediante `BuyUpgradeRemote` y actualiza los textos de coste y nivel en pantalla.

---

## 6. Lista de Verificacion Final para el Agente

1. Ejecutar `studio status` o verificar conexion del bridge en `http://127.0.0.1:58741/status`.
2. Crear los objetos 3D en `Workspace` usando `build_tree` o `execute_luau`.
3. Inyectar `ClickRemote`, `BuyUpgradeRemote` y `UpgradeConfig` en `ReplicatedStorage`.
4. Inyectar `LeaderstatsManager` y `UpgradeManager` en `ServerScriptService`.
5. Construir `ClickerHUD` en `StarterGui` con su `HUDController`.
6. Realizar un playtest corto o verificar que la tabla de leaderstats aparezca con los valores en 0.
7. Guardar el mapa en `maps/IncrementalClicker.rbxl` mediante `studio save` o solicitando guardar localmente.
