import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import ts from "typescript";

const THRESHOLD = Number(process.env.MUTATION_THRESHOLD ?? 80);
const SRC_DIR = path.resolve(import.meta.dirname, "../src");

const tokenSwaps = new Map([
  [ts.SyntaxKind.PlusToken, "+ -> -"],
  [ts.SyntaxKind.MinusToken, "- -> +"],
  [ts.SyntaxKind.EqualsEqualsEqualsToken, "=== -> !=="],
  [ts.SyntaxKind.ExclamationEqualsEqualsToken, "!== -> ==="],
  [ts.SyntaxKind.LessThanToken, "< -> <="],
  [ts.SyntaxKind.LessThanEqualsToken, "<= -> <"],
  [ts.SyntaxKind.GreaterThanToken, "> -> >="],
  [ts.SyntaxKind.GreaterThanEqualsToken, ">= -> >"],
  [ts.SyntaxKind.AmpersandAmpersandToken, "&& -> ||"],
  [ts.SyntaxKind.BarBarToken, "|| -> &&"],
]);

const replacementText = new Map([
  [ts.SyntaxKind.PlusToken, "-"],
  [ts.SyntaxKind.MinusToken, "+"],
  [ts.SyntaxKind.EqualsEqualsEqualsToken, "!=="],
  [ts.SyntaxKind.ExclamationEqualsEqualsToken, "==="],
  [ts.SyntaxKind.LessThanToken, "<="],
  [ts.SyntaxKind.LessThanEqualsToken, "<"],
  [ts.SyntaxKind.GreaterThanToken, ">="],
  [ts.SyntaxKind.GreaterThanEqualsToken, ">"],
  [ts.SyntaxKind.AmpersandAmpersandToken, "||"],
  [ts.SyntaxKind.BarBarToken, "&&"],
]);

function collectSourceFiles(dir) {
  const results = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      results.push(...collectSourceFiles(full));
    } else if (
      entry.name.endsWith(".ts") &&
      !entry.name.endsWith(".test.ts") &&
      !entry.name.endsWith(".d.ts")
    ) {
      results.push(full);
    }
  }
  return results;
}

function collectMutantsForFile(filePath) {
  const sourceText = fs.readFileSync(filePath, "utf8");
  const sf = ts.createSourceFile(
    filePath,
    sourceText,
    ts.ScriptTarget.Latest,
    true,
  );
  const mutants = [];

  function visit(node) {
    if (ts.isBinaryExpression(node)) {
      const opKind = node.operatorToken.kind;
      if (tokenSwaps.has(opKind)) {
        const start = node.operatorToken.getStart(sf);
        const end = node.operatorToken.getEnd();
        const repl = replacementText.get(opKind);
        const { line, character } = sf.getLineAndCharacterOfPosition(start);
        mutants.push({
          filePath,
          line: line + 1,
          col: character + 1,
          desc: tokenSwaps.get(opKind),
          originalText: sourceText,
          mutatedText:
            sourceText.slice(0, start) + repl + sourceText.slice(end),
        });
      }
    }
    ts.forEachChild(node, visit);
  }

  visit(sf);
  return mutants;
}

const files = collectSourceFiles(SRC_DIR);
const allMutants = files.flatMap(collectMutantsForFile);

if (allMutants.length === 0) {
  console.log("ts-mutate: no mutable expressions found");
  process.exit(0);
}

let killed = 0;
for (const m of allMutants) {
  const rel = path.relative(process.cwd(), m.filePath);
  try {
    fs.writeFileSync(m.filePath, m.mutatedText, "utf8");
    try {
      execFileSync("npx", ["vitest", "run", "--config", "vitest.config.ts"], {
        stdio: "ignore",
      });
      console.log(`[SURVIVED] ${rel}:${m.line}:${m.col} (${m.desc})`);
    } catch {
      killed++;
      console.log(`[KILLED]   ${rel}:${m.line}:${m.col} (${m.desc})`);
    }
  } finally {
    fs.writeFileSync(m.filePath, m.originalText, "utf8");
  }
}

const score = (killed * 100) / allMutants.length;
console.log(
  `TS Mutation score: ${score.toFixed(1)}% (${killed} / ${allMutants.length} killed, threshold ${THRESHOLD.toFixed(1)}%)`,
);
if (score < THRESHOLD) {
  process.exit(1);
}
