/**
 * Interseguro Coding Challenge – API de estadísticas de Node.js
 * Framework : Express.js
 * Port      : 3001 (default)
 *
 * Recibe las matrices Q y R de la API de Go y devuelve:
 * un resumen estadístico: máximo, mínimo, promedio, suma y verificación de la diagonal.
 */

'use strict';

const express = require('express');
const cors    = require('cors');

const app  = express();
const PORT = process.env.PORT || 3001;

// ─────────────────────────────────────────────
//  MIDDLEWARE
// ─────────────────────────────────────────────

app.use(express.json());
app.use(cors());

// Simple request logger
app.use((req, _res, next) => {
  console.log(`[${new Date().toISOString()}] ${req.method} ${req.url}`);
  next();
});

// ─────────────────────────────────────────────
//  HELPER FUNCTIONS
// ─────────────────────────────────────────────

/**
 * Flattens a 2-D matrix into a 1-D array of numbers.
 * @param {number[][]} matrix
 * @returns {number[]}
 */
const flatten = (matrix) => matrix.flat();

/**
 * Determines whether a matrix is diagonal.
 * A matrix is diagonal when it is square and all off-diagonal elements are zero.
 *
 * @param {number[][]} matrix
 * @returns {{ isDiagonal: boolean, reason: string }}
 */
const checkDiagonal = (matrix) => {
  const m = matrix.length;
  if (m === 0) return { isDiagonal: true, reason: 'empty matrix' };

  const n = matrix[0].length;
  if (m !== n) {
    return { isDiagonal: false, reason: `not square (${m}×${n})` };
  }

  for (let i = 0; i < m; i++) {
    for (let j = 0; j < n; j++) {
      if (i !== j && Math.abs(matrix[i][j]) > 1e-10) {
        return {
          isDiagonal: false,
          reason: `non-zero off-diagonal element at [${i}][${j}] = ${matrix[i][j]}`,
        };
      }
    }
  }
  return { isDiagonal: true, reason: 'all off-diagonal elements are zero' };
};

/**
 * Computes aggregate statistics over all values in the provided named matrices.
 *
 * @param {Object.<string, number[][]>} matrices  e.g. { Q: [[...]], R: [[...]] }
 * @returns {{
 *   aggregate: { max, min, average, totalSum, elementCount },
 *   diagonalAnalysis: Object
 * }}
 */
const computeStatistics = (matrices) => {
  const allValues = Object.values(matrices).flatMap(flatten);

  if (allValues.length === 0) throw new Error('no numeric values found in matrices');

  const max = Math.max(...allValues);
  const min = Math.min(...allValues);
  const sum = allValues.reduce((acc, v) => acc + v, 0);
  const avg = sum / allValues.length;

  // Round to 8 decimal places to avoid floating-point noise in output
  const r8 = (v) => parseFloat(v.toFixed(8));

  // Check diagonal condition for each individual matrix
  const diagonalAnalysis = Object.fromEntries(
    Object.entries(matrices).map(([key, mat]) => [key, checkDiagonal(mat)])
  );

  return {
    aggregate: {
      max:          r8(max),
      min:          r8(min),
      average:      r8(avg),
      totalSum:     r8(sum),
      elementCount: allValues.length,
    },
    diagonalAnalysis,
  };
};

// ─────────────────────────────────────────────
//  ROUTES
// ─────────────────────────────────────────────

/** GET /health — liveness probe */
app.get('/health', (_req, res) => {
  res.json({ status: 'ok', service: 'node-api' });
});

/**
 * POST /api/v1/statistics
 *
 * Body   : { Q: number[][], R: number[][] }
 * Returns: aggregate statistics and per-matrix diagonal analysis.
 *
 * This endpoint is called internally by the Go API after performing QR
 * decomposition. It can also be called directly for standalone testing.
 */
app.post('/api/v1/statistics', (req, res) => {
  const { Q, R } = req.body;

  // ── Input validation ─────────────────────────────────────
  if (!Q || !R) {
    return res.status(400).json({
      success: false,
      message: 'Request body must include both Q and R matrices.',
    });
  }
  if (!Array.isArray(Q) || !Array.isArray(R)) {
    return res.status(400).json({
      success: false,
      message: 'Q and R must be 2-D arrays.',
    });
  }

  // ── Computation ──────────────────────────────────────────
  try {
    const data = computeStatistics({ Q, R });
    return res.json({
      success: true,
      message: 'Statistics computed successfully.',
      data,
    });
  } catch (err) {
    console.error('[ERROR]', err.message);
    return res.status(500).json({ success: false, message: err.message });
  }
});

// ─────────────────────────────────────────────
//  START SERVER
// ─────────────────────────────────────────────

app.listen(PORT, () => {
  console.log(`Node.js Statistics API running on port ${PORT}`);
});
