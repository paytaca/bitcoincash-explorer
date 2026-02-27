import { bchRpc } from "../../../utils/bchRpc";

export default defineEventHandler(async (event) => {
  try {
    // Get the current tip
    const blockCount = await bchRpc("getblockcount");
    const tip = Number(blockCount);

    if (Number.isNaN(tip)) {
      throw new Error("Invalid block count from RPC");
    }

    // Fetch last 15 blocks
    const heights = Array.from({ length: 15 }, (_, i) => tip - i);
    const blocks = [];

    for (const height of heights) {
      try {
        const hash = await bchRpc("getblockhash", [height]);
        const block = await bchRpc("getblock", [hash, 2]); // verbosity 2 for tx details

        blocks.push({
          hash: block.hash,
          height: block.height,
          time: block.time,
          size: block.size,
          txCount: Array.isArray(block.tx) ? block.tx.length : 0,
          miner: extractMinerFromBlock(block),
        });
      } catch (e) {
        console.error(`Failed to fetch block at height ${height}:`, e);
        // Continue with other blocks
      }
    }

    return blocks;
  } catch (error) {
    console.error("Error fetching latest blocks:", error);
    throw createError({
      statusCode: 500,
      statusMessage: "Failed to fetch latest blocks",
    });
  }
});

function extractMinerFromBlock(b: any): string | undefined {
  const tx0 = Array.isArray(b?.tx) ? b.tx[0] : undefined;
  const vin0 = Array.isArray(tx0?.vin) ? tx0.vin[0] : undefined;
  const coinbaseHex =
    typeof vin0?.coinbase === "string" ? vin0.coinbase : undefined;
  return extractMinerFromCoinbaseHex(coinbaseHex);
}

function extractMinerFromCoinbaseHex(
  coinbaseHex?: string
): string | undefined {
  if (!coinbaseHex || typeof coinbaseHex !== "string") return undefined;
  if (!/^[0-9a-fA-F]+$/.test(coinbaseHex) || coinbaseHex.length % 2 !== 0)
    return undefined;

  try {
    // Node SSR path
    // eslint-disable-next-line n/no-deprecated-api
    const buf = Buffer.from(coinbaseHex, "hex");
    const ascii = buf.toString("latin1").replace(/[^\x20-\x7E]+/g, " ");
    const cleaned = ascii.replace(/\s+/g, " ").trim();

    if (!cleaned) return undefined;

    // Best-effort: take first slash-delimited tag as "pool" name.
    const m = cleaned.match(
      /\/\s*([A-Za-z0-9][A-Za-z0-9 ._-]{0,40}?)\s*\//
    );
    if (m?.[1]) return m[1].trim();
    return undefined;
  } catch {
    return undefined;
  }
}
