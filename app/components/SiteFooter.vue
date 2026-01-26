<template>
  <footer class="footer">
    <div class="inner">
      <div class="left muted">
        <a class="link" href="https://github.com/paytaca/bitcoincash-explorer" target="_blank" rel="noreferrer">GitHub</a>
      </div>
      <div class="right muted">
        Made with ❤️ by <a class="link" href="https://www.paytaca.com" target="_blank" rel="noreferrer">&nbsp;Paytaca</a>
      </div>
    </div>
  </footer>
</template>

<style scoped>
.footer {
  margin-top: 28px;
  border-top: 1px solid var(--color-border);
  background: var(--color-bg);
}
.inner {
  max-width: 960px;
  margin: 0 auto;
  padding: 18px 16px 26px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.left,
.right {
  display: flex;
  align-items: center;
}
.muted {
  color: var(--color-text-muted);
  font-size: 13px;
}
.link {
  color: var(--color-link);
  text-decoration: none;
  font-weight: 600;
}
.link:hover {
  color: var(--color-link-hover);
  text-decoration: underline;
}
</style>

