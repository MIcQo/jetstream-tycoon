# Jetstream Tycoon — Game Vision (Phase 1)

A single-player, real-time airline tycoon on a simple 2D world map.

## Decisions made
| Topic | Decision |
|---|---|
| Platform | Web: Go backend (authoritative sim) + JS/TS frontend |
| Time | Real-time, pause / 1x / 2x / 4x |
| Players | Single player; architecture ready for AI/multiplayer later |
| Goal | Sandbox + milestones/achievements |
| Graphics | Flat 2D map (SVG/canvas): dots = airports, lines = routes, small icons = planes |

## Core loop
Earn money → buy territory → build airport → open route → assign planes → flights earn → upgrade → repeat.

## Entities
- **Territory**: a region of the map. It has a purchase price, a demand factor (population), and a profile (tourism vs. business). You must own a territory to build airports in it.
- **Airport**: lives in a territory and has a level from 1 to 5. The level sets passenger capacity per day, the number of route slots, and the largest plane size it can handle. It has upkeep and upgrade costs.
- **Plane model**: comes in tiers (Prop → Regional → Narrowbody → Widebody → Jumbo), each with seats, range, speed, price and upkeep.
- **Plane**: one owned instance of a model, with a level (1–5) that raises seats, fuel efficiency and reliability. It is assigned to one route.
- **Route**: a link between two owned airports. It can hold many planes of different tiers and levels. Demand comes from both territories, and the distance must be within each plane's range.
- **Flight**: one plane flying one leg of a route. Revenue = passengers × ticket price − fuel − airport fees. Reliability decides whether a flight succeeds or is delayed/cancelled (no or reduced income).

## Random events
Events fire on game ticks, with weighted chances and conditions (for example, "player owns an airport in Asia").
- Positive: a festival raises demand in a territory, fuel gets cheaper, a tourism boom.
- Negative: a storm closes an airport, a strike, a fuel price spike, a plane breaks down.
- Some events ask for a decision, e.g. "Pay $X to end the strike now, or wait 5 days?"

## Investments (offers)
Offers pop up from time to time. You can accept or decline them, and they expire.
- **Ad campaign**: costs $X. 60%: demand +20% for 30 days. 40%: no effect.
- **Fuel hedge**: locks the fuel price. Pays off or loses depending on the market.
- **Partnership**: an upfront fee buys a revenue share on a route.
- **Tech R&D**: may unlock a reliability bonus, or fail.
- Outcomes can be revealed right away or delayed, so you don't know at once whether it worked.

## Economy
- Daily upkeep covers airports and planes. Net worth = cash + assets.
- Optional for later: loans, bankruptcy and selling assets.

## Milestones (examples)
First route · 5 airports · connect all continents · 100 planes · $1B net worth.

## Architecture (for extensibility)
- **Go server**: a fixed-tick simulation loop. A REST API handles actions (buy, upgrade, accept offer), and a WebSocket pushes state and events to the client.
- **Postgres**: stores saves, schema managed with **golang-migrate**. Redis is not needed at first.
- **Data-driven content**: territories, plane models, airport levels, events and investments are defined in YAML/JSON files. New content means a new file, not new code.
- **Effect/modifier system**: every bonus or malus uses one shape: `{target, stat, op, value, duration}`. Events, investments, upgrades and later research all reuse it.
- **Registries**: event types, outcome resolvers and milestone checks plug in through Go interfaces.
- **Deterministic RNG**: seeded per save, which makes testing, replays and later multiplayer easier.
- **Frontend**: thin and rendering-only. All rules live on the server.

## MVP scope
1. World map with real countries and about 1,000 real airports
1b. Ticket pricing, attractiveness and reputation
1c. Loans, credit rating and bankruptcy
2. Buy territory, build or upgrade an airport
3. 3 plane tiers, buy or upgrade planes, create routes and assign planes
4. Real-time flights with revenue
5. About 8 random events and 3 investment types
6. Save/load and 5 milestones

## Decisions (round 2)
| Topic | Decision |
|---|---|
| Territories | Real countries (~190) |
| Airports | Real cities and airports |
| Ticket prices | Set by the player per route; price drives airline attractiveness |
| Loans and bankruptcy | Included in the MVP |

## World data
- **Countries**: shapes from Natural Earth (public domain), simplified for a lightweight SVG map.
- **Airports**: from OurAirports (public domain), filtered to large and medium airports (~1,000). Each airport has lat/lon, city and IATA code.
- **Country stats**: population and GDP per capita set demand, purchase price and passenger wealth.
- **Purchase price** = f(GDP, population). Small countries are cheap starters, large ones are late-game goals.
- A country must be owned before you can build at its airports. An airport starts at level 0 (unbuilt); you pay to activate it.

## Ticket pricing and attractiveness
- Each route has a **reference price** = base + distance × rate × country wealth factor.
- The player sets the **ticket price** for each route.
- **Attractiveness** (0–2) = price factor × service factor × reputation:
  - Price factor: cheaper than the reference price raises it, more expensive lowers it (elastic curve, clamped).
  - Service factor: average plane level and age on the route, plus airport level.
  - Reputation: global airline score, raised by on-time flights and lowered by cancellations and events.
- **Passengers per flight** = min(seats, route demand × attractiveness ÷ flights per day).
- Trade-off: a low price fills seats on thin margins, a high price brings fewer, richer passengers.
- The UI shows the expected load factor and profit per flight as the price slider moves.
- The model has room for AI competitors later: attractiveness becomes a share against rival airlines.

## Loans and bankruptcy
- **Bank loan offers**: amount, interest rate and term. The rate depends on your **credit rating** (based on net worth, debt ratio and payment history).
- **Repayment**: automatic weekly instalments, with an option to repay early.
- **Overdraft**: cash may go negative briefly, at a high penalty interest.
- **Warning stages**:
  1. Cash < 0 → warning.
  2. Missed payment → credit rating drops, interest rises.
  3. Negative for N days → **forced asset sales** (planes first, then airports) to cover the debt.
  4. Nothing left to sell and debt remains → **bankruptcy** = game over. The save stays viewable, with a "restart" option.
- Loan offers can also appear as investment events (e.g. "Government subsidy loan at 1% if you open 3 routes in Africa").

## Decisions (round 3)
| Topic | Decision |
|---|---|
| Start | Player picks a home country |
| Time | 1 in-game day = 60 s at 1x |
| Plane aging | Yes, with a second-hand market |
| Fuel | Fluctuating global price |

## Game start
- On a new game, the player picks a **home country** and an **airline name**.
- **Starting kit**: the home country (free), its main airport at level 1 as the hub, one starter plane, and starting cash.
- **Balance**: starting cash scales inversely with the home country's value. A small country gets more cash; a big country gets rich demand but less cash.
- The first loan offer is available immediately.

## Time
- Tick = 1 in-game hour (2.5 s at 1x). Speeds: pause, 1x, 2x, 4x.
- Flights resolve per tick. Upkeep, loan payments and fuel price update once per day. Monthly report every 30 days.

## Plane aging and second-hand market
- Each plane has an **age** (in game days) and **condition** (0–100%).
- Condition falls with every flight and with time.
  - Low condition: higher maintenance cost, lower reliability, lower attractiveness.
  - **Overhaul**: costs money and takes the plane out of service for N days; restores condition (but not age).
- **Resale value** = model price × level bonus × depreciation(age) × condition.
- **Used-plane market**: a rotating list of cheaper, older planes for sale, refreshed weekly. The player can sell planes into it.
- Planes past their maximum age are unsafe: big reliability penalty, and they trigger negative events.

## Fuel market
- Global fuel price per day follows a mean-reverting random walk with occasional shocks.
- Shocks come from events (e.g. "Oil crisis: +60% for 20 days").
- A 90-day fuel price chart is shown in the UI.
- Fuel cost per flight = distance × model burn × (1 − efficiency bonus from level) × fuel price.
- A **fuel hedge** investment locks the price for N days: profit if the market rises, loss if it falls.

## Decisions (round 4)
| Topic | Decision |
|---|---|
| Platforms | Web + iOS + Android (Capacitor, shared codebase) |
| Offline | Fully playable offline; cloud sync when online |
| Accounts | Guest first, link Apple/Google/email |
| Metrics | Gameplay + product analytics in ClickHouse |

## Decisions (round 5)
| Topic | Decision |
|---|---|
| Monetization | Free with ads (rewarded first, few interstitials) |
| MVP additions | Competition, hubs + connecting passengers, missions, tutorial, restructuring |
| Also in scope | Seasonal demand, seat classes, landing rights |

## Competition (virtual competitors)
> Rev 6: these virtual competitors remain as **background carriers** next to the real AI airlines (see *AI airlines*).
- Each route has 0–N **virtual competitor airlines** with fictional names, generated per region. Popular routes have more of them.
- Each competitor has a price, a quality and a seat capacity.
- **Market share** = your attractiveness ÷ (yours + the sum of competitors').
  - An empty route = monopoly: share is limited only by demand and the price curve.
- Competitors react slowly:
  - They drift toward the reference price.
  - They undercut you if you dominate the route.
  - They can start a **price war** event, or leave a route where they keep losing.
- This is designed so full AI airlines can replace the virtual ones later.

## Hubs and connecting passengers
- Your own airport at **level ≥ 3** can be designated a **hub** (the number of hubs is limited and unlocked by milestones).
- If you fly O→H and H→D, where H is a hub and there's no direct O–D route, you get **connecting passengers** between O and D.
- Connecting demand = O–D demand × connect factor (≈0.4) × the lower attractiveness of the two legs.
- Connecting passengers use seats on **both legs**. Direct passengers board first, then connecting ones fill the remaining seats.
- A connecting passenger pays (leg 1 + leg 2) × 0.85, a transfer discount.
- The MVP allows one transfer; two transfers can be added later.
- The UI shows the top connecting flows through each hub.

## Missions and contracts
- A **mission board** with 3 active slots refreshes every few days.
  - Examples: "Connect X–Y within 30 days", "Carry 50k passengers to South America this quarter", "Keep load factor > 80% on route Z for 2 weeks".
- Each mission has a reward (cash, reputation, a free plane, a hub slot) and an optional penalty if it fails.
- Missions come from templates in content files and are filled with real airports near your network.
- Story-like **government contracts** (subsidised routes) are a mission type too.

## Tutorial (first 10 minutes)
- A guided sequence:
  1. Explore the home country.
  2. Buy the first plane.
  3. Open the first domestic route.
  4. Set the price (with the forecast).
  5. Watch the first profitable day.
  6. Buy landing rights abroad.
  7. Take the first mission.
- The steps are data-driven (each step has highlighted UI and a completion condition). They can be skipped or replayed.

## Restructuring (soft bankruptcy)
- The first time you reach bankruptcy, you get a **restructuring** offer instead of game over:
  - Debt is converted into one long-term loan at a high rate.
  - The lowest-value half of the fleet is sold.
  - Non-hub airports lose one level.
  - Reputation −30 and credit score reset to low.
  - The home country and hub are kept.
- The second bankruptcy = game over.
- The number of restructurings allowed is a balance setting.

## Seasonal demand
- **Seasons**: a monthly demand curve per country based on hemisphere, climate and tourism profile. For example, Mediterranean summer peaks and southern-hemisphere peaks in Dec–Feb.
- **Holidays**: a calendar of named periods (Christmas/New Year, Chinese New Year, Ramadan/Eid, Diwali, Thanksgiving, school holidays) that raise demand in the affected countries.
- **Business demand** dips in holidays; **tourism demand** rises.
- The route panel shows a 12-month demand outlook, so players can plan capacity and pricing.

## Seat classes
- Two classes: **Economy** and **Business**, with separate ticket prices on each route.
- Each plane has a seat configuration: one business seat uses 2 economy seat spaces. Reconfiguring costs money and takes 2 days of downtime.
- Business demand share depends on the countries' business profile and GDP (about 5–20%).
- Business reference price ≈ 3.5 × economy. Business passengers react less to price but care more about service (plane level and condition).
- Later: Premium economy / First class, defined in content.

## Landing rights
- A cheaper way into foreign markets: **lease landing rights** at one airport in a country you don't own.
  - Cost: an upfront fee (≈5–15% of the country price, scaled by airport size) plus a **yearly lease**.
- Limitations compared with owning the country:
  - The airport keeps its real-world base level; you can't upgrade it.
  - Higher fees per passenger.
  - No hub.
  - Limited number of daily departures.
- Owning a country: lower fees, upgrades, hubs, all of its airports.
- Lease contracts expire if unpaid, and events can revoke or discount them.

## Monetization: free with ads
- **Rewarded ads** (opt-in, the main income source):
  - 2x revenue for 1 game day.
  - Instant cash grant (scaled to your size).
  - Halve overhaul downtime.
  - Reveal investment odds.
  - An extra mission reroll.
- **Interstitials**: at most one per ~10 min, only at natural breaks (monthly report screen). Never during decisions, never in the tutorial.
- No banners on the map.
- Optional later: a "Remove ads" in-app purchase.
- Offline: rewarded buttons are hidden, and the game is fully playable without ads.

## Decisions (round 6)
| Topic | Decision |
|---|---|
| AI airlines | Full AI competitor airlines playing by the same rules |
| Country ownership | Exclusive (one owner); buyout offers possible |
| Difficulty | Easy / Normal / Hard at new game; AI personalities |
| Takeovers | Both ways; AI airlines can go bankrupt |

## AI airlines
- **2–6 rival airlines**, depending on difficulty. Each has a fictional name, a colour, a home country and a **personality**: Budget, Premium, Regional, Aggressive or Conservative.
- They buy countries, lease landing rights, open routes, buy and upgrade planes, set prices, take loans and react to events, **using exactly the same actions and rules as you**.
- **Competition**: on any route, passengers split between every airline flying it plus background carriers, based on attractiveness (price, service, reputation).
  - Rivals react to your price changes (faster on Hard) and can start price wars.
- **Countries are exclusive.**
  - If a rival owns a country, you can still fly there with landing rights; the owner earns part of your fees.
  - You can also make a **buyout offer**. Rivals sell more easily when they are in financial trouble, and never sell their home country.
  - Rivals can make offers to you too.
- **Takeovers**:
  - You can buy out a struggling rival and absorb its fleet, routes and debt.
  - If you're in trouble, a rival may offer to buy you. Accepting ends the game with a "Sold" ending and a score, as an alternative to restructuring.
  - Rivals can go bankrupt: their planes flood the used market and their countries become available.
- **Visibility**: a rivals ranking panel, rival colours on the map, and a news feed of rival moves. Missions and milestones involve rivals ("Become #1", "First takeover").
- **Difficulty**: Easy has 2 sloppier rivals, Normal 4, Hard 6 sharp ones who react quickly.

## Status
Phase 1 (vision) is complete. Next: Phase 2, a technical design covering data schemas, simulation formulas and the API, followed by an MVP implementation plan.
