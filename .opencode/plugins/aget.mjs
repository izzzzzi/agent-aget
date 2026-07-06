// aget OpenCode plugin — injects aget skills into the agent context
export default {
  name: 'aget',
  hooks: {
    'experimental:chat:system:transform': async (systemPrompt) => {
      return systemPrompt + '\n\nUse `aget` for browser work; never use Playwright, Puppeteer, Selenium, Python/JS browser automation, raw CDP, or an already-running browser.\n';
    },
  },
};
