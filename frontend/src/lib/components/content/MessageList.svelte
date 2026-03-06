<!-- ABOUTME: Renders an array of messages with content rendering components. -->
<!-- ABOUTME: Passes developer and agent name context through to each MessageContent. -->
<script lang="ts">
  import type { MessageWithToolCalls } from "../../api/types.js";
  import MessageContent from "./MessageContent.svelte";

  interface Props {
    messages: MessageWithToolCalls[];
    developerName?: string;
    agentName?: string;
    highlightOrdinal?: number | null;
  }

  let { messages, developerName, agentName, highlightOrdinal = null }: Props = $props();
</script>

<div class="message-list">
  {#each messages as message (message.ordinal)}
    <div data-ordinal={message.ordinal}>
      <MessageContent {message} {developerName} {agentName} expandTools={highlightOrdinal === message.ordinal} highlight={highlightOrdinal === message.ordinal} />
    </div>
  {/each}
</div>

<style>
  .message-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 8px 0;
  }

</style>
