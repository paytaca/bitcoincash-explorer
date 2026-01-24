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
  border-top: 1px solid rgba(17, 24, 39, 0.08);
  background: rgba(249, 250, 251, 1);
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
  color: rgba(107, 114, 128, 1);
  font-size: 13px;
}
.link {
  color: rgba(29, 78, 216, 1);
  text-decoration: none;
  font-weight: 600;
}
.link:hover {
  text-decoration: underline;
}
</style>

