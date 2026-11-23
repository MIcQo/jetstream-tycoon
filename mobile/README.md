# Mobile shell (Capacitor)

iOS and Android apps that package the shared web build (`web/dist`) with Capacitor (ADR-0003).
App ID: `io.github.micqo.jetstreamtycoon`.

```bash
cd mobile
npm install
npm run sync          # builds sim.wasm + the web app, then copies it into ios/ and android/
npm run open:android  # Android Studio
npm run open:ios      # Xcode (macOS)
```

- Native projects (`ios/`, `android/`) are committed. The copied web assets and build outputs are git-ignored.
- Platform-specific code lives only in `web/src/platform/` behind the interfaces in `types.ts`. ESLint blocks Capacitor imports anywhere else.
- Implemented now: the app lifecycle (the game pauses when the app goes to the background).
- Planned: Filesystem saves (M2), secure tokens and sign-in (M9), AdMob, consent and local notifications (M11).
