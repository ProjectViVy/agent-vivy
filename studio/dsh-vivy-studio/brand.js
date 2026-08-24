(() => {
  const PRODUCT = "Vivy Studio"
  const rename = () => {
    const raw = document.title || ""
    if (raw === "") {
      document.title = PRODUCT
      return
    }
    if (raw.includes("DeepSeek Harness")) {
      document.title = raw.replaceAll("DeepSeek Harness", PRODUCT)
    }
  }
  rename()
  const title = document.querySelector("title")
  if (title) {
    new MutationObserver(rename).observe(title, {
      childList: true,
      characterData: true,
      subtree: true,
    })
  }
})()
