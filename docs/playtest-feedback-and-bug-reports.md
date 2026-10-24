# Playtest Feedback and Bug Reports

- Date: 2026-10-24
- Context: Early-to-mid game playtest session covering Lobby (Room 1), Orrery Wing (Room 2), and Deep Space Telescope (Room 3).

---

## 1. Visual and World Geometry Bugs

### Environment & Floor Issues
- **Floor rock clipping & texture swimming**: Rocks pierce through the floor geometry. As the player moves, the texture shifts and swims unnaturally.
- **Overlapping floor geometry**: Double marble floors overlapping one another; visible z-fighting and misaligned floor edges.
- **Spyglass table model**:
  - The 3D model of the brass spyglass resting on the lectern/table is physically misaligned.
  - The table contains surface text: *"Stargazer's Guide" take the spyglass from*.
  - The text orientation is rotated sideways relative to the player approaching angle, making it difficult to read.
- **Lobby to Orrery doorway design**:
  - A structural column/pillar sits directly in the middle of the doorway opening, blocking visual flow and movement path.
- **Deep space telescope model (Room 3)**:
  - Several components of the large telescope model do not connect or touch each other properly, leaving floating/misaligned sections.

---

## 2. UI and Audio Bugs

### Navigation & Borders
- **C Menu navigation buttons**: Top-left tab buttons (*Star Catalog*, *Lenses*, *How to Play*) have their top and left borders clipped by container padding. The bottom and right borders render correctly.
- **Camera cinematic occluded by UI**: When triggering door unlocks or astral shifts from inside the menu, the cinematic camera pan plays in the background while the menu remains open over it, preventing the player from seeing the unsealing sequence.

### HUD & Resource Counters
- **Currency icon centering**:
  - Starlight (star counter) icon is offset down and to the right inside its circular frame.
  - General currency icons appear misaligned within their background circles, likely caused by Unicode glyph font metrics and bounding box padding. Recommendation: replace Unicode characters with dedicated ImageLabels or vector assets.
- **Missing icon for Tier 3 currency**: The third resource (purple) displays an empty solid purple circle with no inner glyph or icon.

### Audio Feedback
- **Sound drop under rapid purchase**: Purchasing upgrades or items in quick succession occasionally drops the purchase sound effect.
- **Repetitive starfall audio**: The starfall event audio plays too frequently (up to 20 times in a short window) and becomes irritating over time.

---

## 3. Gameplay Clarity and Level Design

### Objectives & Onboarding
- **Initial objective ambiguity**: The player is not clearly guided toward opening the Orrery door. The door opens suddenly without warning or a visible progress indicator.
- **Door cost indication**: Recommend placing a clear, readable physical sign or progress meter at the door displaying the required currency amount so the player understands the goal early on.
- **Starfall explanation**: Unclear what the starfall event actually does. Needs an initial pop-up explanation or a clearer breakdown in the *How to Play* guide (especially for younger players).
- **Constellation / Astral Shift introduction**: The Constellations tab unlocks without onboarding or context. Players are unsure what shifting or shards achieve until accidental experimentation.

### Pacing & Automation Balance
- **Early automation overpower**: Upgrading Brass Clockwork Gears to tier 8 quickly yields over 35,000 starlight automatically. This trivializes the active stargazing mechanic too early in the progression loop.

---

## 4. Feature and Game Design Suggestions

### Physical World Interaction vs Menu-Only Upgrades
- **In-world interaction stations**: Currently, all upgrades are conducted through the 2D "C" menu. In a 3D environment, upgrades should encourage physical movement throughout the observatory (e.g., walking up to the Orrery apparatus to upgrade it rather than clicking a menu).
- **Interactive cosmic dust loop**: Pressing E repeatedly to collect cosmic dust feels tedious. At minimum, it should support holding E down. Ideally, replace the button-press generator with an interactive repair or calibration mechanic centered around the 3D Orrery.
- **Overarching game theme**: Frame progression around restoring and repairing an ancient neglected observatory:
  - *Room 1*: Start with a weathered spyglass, discovering and cataloging basic stars in a field notebook.
  - *Room 2*: Repair the grand Orrery mechanism by scanning passing planets (Moon, Mars, Jupiter) and integrating them into the model.
  - *Room 3*: Unlock high-magnification deep space lenses to study distant cosmic phenomena.

### Stargazing Scope Mechanics & Viewport
- **Spyglass un-aimed visibility**: Question whether celestial bodies should be prominently visible with the naked eye. Consider keeping them faint or hidden until viewed through optic lenses.
- **Skybox integration**: Better harmonize ambient backdrop stars and the moon with targetable gameplay stars.
- **Spyglass skill progression**: Introduce natural handling improvements (reduced optic sway) as player levels up or logs more observations, rather than relying solely on Shift steadying.
- **Viewport screen real estate**: While aiming through the spyglass, large areas of the screen remain unused:
  - Display observational data, astronomical classifications, or field notes.
  - Progressive discovery: Scanning an object multiple times unveils additional tiers of information (composition, gravity, history, trivia).
  - Expand targets beyond stars: Moons (e.g. Moon tutorial with Apollo landing facts), asteroids, comets, planets, pulsars, and black holes to provide educational and discovery value for kids.
- **Room 3 Deep Space Telescope**:
  - Interacting with the Void Loom currently yields a temporary Frenzy buff. The visual setup leads players to expect a high-tier stationary telescope with enhanced zoom, eliminated sway, or unique deep-space targets.
