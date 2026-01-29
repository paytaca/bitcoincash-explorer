<template>
  <footer class="footer">
    <div class="inner">
      <div class="left muted">
        <a class="link" href="https://github.com/paytaca/bitcoincash-explorer" target="_blank" rel="noreferrer">GitHub</a>
      </div>
      <div class="center muted">
        Made with ❤️ by <a class="link" href="https://www.paytaca.com" target="_blank" rel="noreferrer">&nbsp;Paytaca</a>
      </div>
      <div class="right muted">
        <NuxtLink class="link" to="/status">Status</NuxtLink>
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
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: 12px;
}
.left,
.center,
.right {
  display: flex;
  align-items: center;
}
.center {
  justify-content: center;
  text-align: center;
}
.right {
  justify-content: flex-end;
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

@media (max-width: 520px) {
  .inner {
    grid-template-columns: 1fr;
    gap: 8px;
    justify-items: center;
  }
  .left,
  .center,
  .right {
    justify-content: center;
  }
}
</style>

