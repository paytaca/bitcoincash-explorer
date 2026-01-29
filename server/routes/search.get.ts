export default defineEventHandler(async (event) => {
  const q = String(getQuery(event)?.q ?? '').trim()
  if (!q) {
    return sendRedirect(event, '/', 302)
  }

  const txid = q.toLowerCase().replace(/^0x/, '')
  if (/^[0-9a-f]{64}$/.test(txid)) {
    return sendRedirect(event, `/tx/${txid}`, 302)
  }

  // Lightweight cashaddr-like check (prefix optional); server address API will validate fully.
  const v = q.toLowerCase()
  const hasPrefix = v.includes(':')
  const prefix = hasPrefix ? v.split(':', 1)[0] : ''
  const payload = hasPrefix ? v.split(':').slice(1).join(':') : v
  const okPrefix = !hasPrefix || ['bitcoincash', 'bchtest', 'bchreg'].includes(prefix)
  const okPayload = /^[qpzr][0-9a-z]{20,}$/i.test(payload)

  if (okPrefix && okPayload) {
    return sendRedirect(event, `/address/${encodeURIComponent(q)}`, 302)
  }

  // Unknown input → go home (could be improved later with an error query param)
  return sendRedirect(event, '/', 302)
})

