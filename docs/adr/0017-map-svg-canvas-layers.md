# ADR-0017: World map rendered as SVG base + Canvas overlay + HTML labels

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
The map shows about 190 countries, about 1,000 airports, many routes and hundreds of moving planes inside mobile WebViews.

## Decision
- The **SVG** layer has countries (TopoJSON, pre-projected path strings from datagen, two detail levels) and airports, and handles hit-testing.
- The **Canvas** layer draws route arcs and planes with requestAnimationFrame, DPR-aware, culled to the viewport and capped at 30 fps on mobile.
- An **HTML** layer handles labels and popups.
- All layers share one projection and transform.
- Plane positions are interpolated on the client from depart and arrive times.

## Consequences
- Simple 2D graphics with smooth mobile performance and accessible patterns for colour-blind players.
- Two rendering techniques to keep in sync.

## Alternatives considered
Pure SVG (too slow with many moving planes); WebGL/Pixi (more than needed); a map tile library such as Leaflet/Mapbox (heavy, online tiles).
