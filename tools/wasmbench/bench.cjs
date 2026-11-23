const { chromium } = require('playwright');
const http = require('http'); const fs = require('fs'); const path = require('path');
const dir = process.argv[2];
const srv = http.createServer((q, r) => { const f = path.join(dir, q.url.split('?')[0]); const ct = f.endsWith('.wasm') ? 'application/wasm' : f.endsWith('.js') ? 'text/javascript' : 'text/html'; fs.readFile(f, (e, d) => { if (e) { r.writeHead(404); r.end(); } else { r.writeHead(200, { 'Content-Type': ct }); r.end(d); } }); }).listen(4180);
(async () => {
  const b = await chromium.launch();
  for (const rate of [1, 4, 6]) {
    const runs = [];
    for (let i = 0; i < 3; i++) {
      const p = await b.newPage();
      const cdp = await p.context().newCDPSession(p);
      await cdp.send('Emulation.setCPUThrottlingRate', { rate });
      await p.goto('http://localhost:4180/bench.html');
      runs.push(await p.evaluate(() => window.bench()));
      await p.close();
    }
    const med = k => runs.map(r => r[k]).sort((a, b) => a - b)[1].toFixed(1);
    console.log(`cpu x${rate}: compile ${med('compileMs')} ms, instantiate+start ${med('instantiateRunMs')} ms, newGame ${med('newGameMs')} ms, 1 game day ${med('dayAvgMs')} ms`);
  }
  await b.close(); srv.close();
})();
