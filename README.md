# FormulaTerm
Live timing CLI dashboard for f1 sessions. Eventually will include telemetry.

## Notes
Generally will not have AI generated code in this project due to using this as a learning experience for Go, and just doing it for the love of the game.
However, AI will probably be used to generate tests/audit code to catch bugs, because there's a lot of foot guns dealing with this API.

## Todo
- [x] Connect and Read from the live timing signalR endpoint
- [x] Figure out the structure of all fields returned
- [ ] Parse fields and expose data via a callback
- [ ] Build the application state pipeline 
- [ ] Build the terminal UI (Status Bar, Timing Table, Driver Details, Live Event Feed)
- [ ] Extract the telemetry data, normalize track positions, add graphs for common metrics 
- [ ] Build the ASCII Track Map

## Outline
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│ F1TERM v0.8 | BEL GP | QUALI Q3 | 08:41 REM | TRACK 34.8°C | AIR 24.1°C | DRY | GREEN        │
├───────────────────────────────┬──────────────────────────────────────────────────────────────┤
│ TIMING                        │ SELECTED DRIVER (VER)                                        │
│                               │                                                              │
│ P  CAR DRIVER     GAP   INT   │ Last Lap     1:24.883                                        │
│───────────────────────────────│ Best Lap     1:24.655   🟪                                   │
│ 1  4   NOR     -----  -----   │ Sector 1     28.214    🟩                                    │
│ 2  81  PIA    +0.041  +0.041  │ Sector 2     31.482    🟨                                    │
│ 3  1   VER    +0.098  +0.057  │ Sector 3     25.187    🟪                                    │
│ 4  16  LEC    +0.231  +0.133  │ Speed Trap   332 km/h                                        │
│ 5  63  RUS    +0.277  +0.046  │ Tyre          Soft (5L)                                      │
│ 6  44  HAM    +0.401  +0.124  │ Fuel Est.     Push                                           │
│                               │                                                              │
│ > indicates selected driver   │ Current delta                                                │
├───────────────────────────────┼──────────────────────────────────────────────────────────────┤
│ TRACK MAP                     │ LIVE EVENT FEED                                              │
│                               │                                                              │
│      ○  ○                     │ 08:42 NOR PB S1                                              │
│   ○        ○                  │ 08:42 VER Traffic T10                                        │
│ ○            ○                │ 08:42 YELLOW S2                                              │
│ ○            ● VER            │ 08:41 PIA PIT OUT                                            │
│ ○            ○                │ 08:40 Track Green                                            │
├───────────────────────────────┴──────────────────────────────────────────────────────────────┤
│ COMMAND > lap NOR | sector | tyre | compare VER | race | telemetry | weather | quit          │
└──────────────────────────────────────────────────────────────────────────────────────────────┘
