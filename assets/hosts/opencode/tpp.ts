// tpp for OpenCode: the same two questions Claude Code gets at its Stop, asked when the
// agent finishes responding. The decision stays in the binary; this file is only transport.
//
// Install: copy to .opencode/plugin/tpp.ts (project) or ~/.config/opencode/plugin/
// Requires: tpp on PATH.
import type { Plugin } from "@opencode-ai/plugin"

export const TPP: Plugin = async ({ $, directory, client }) => {
  return {
    event: async ({ event }) => {
      // session.idle is OpenCode's "the agent finished responding", the analogue of Stop.
      if (event.type !== "session.idle") return
      const result = await $`tpp check --cwd ${directory}`.quiet().nothrow()
      if (result.exitCode === 0) return // nothing owed: stay silent
      const text = result.stdout.toString().trim()
      if (!text) return
      await client.tui.showToast({
        body: { message: "tpp: this change still owes testing. Want feedback on the run?", variant: "warning" },
      })
      await client.tui.appendPrompt({
        body: {
          text:
            "tpp reports what this repository still owes:\n\n" + text +
            "\n\nSay plainly which surfaces went unexamined before calling this done, or close them.",
        },
      })
    },
  }
}
