<script setup lang="ts">
import { useToast } from "../composables/useToast";

const { toasts, dismiss } = useToast();
</script>

<template>
  <Teleport to="body">
    <div class="toast-host" aria-live="polite" aria-atomic="false">
      <TransitionGroup name="toast">
        <div
          v-for="item in toasts"
          :key="item.id"
          class="toast"
          :class="`toast-${item.type}`"
          role="status"
          @click="dismiss(item.id)"
        >
          <span class="toast-icon" aria-hidden="true">
            <svg v-if="item.type === 'success'" width="18" height="18" viewBox="0 0 24 24" fill="none">
              <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.7" />
              <path d="M8.2 12.4l2.6 2.6 5-5.4" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
            <svg v-else-if="item.type === 'error'" width="18" height="18" viewBox="0 0 24 24" fill="none">
              <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.7" />
              <path d="M12 7.5v5.2" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" />
              <circle cx="12" cy="16.4" r="1.05" fill="currentColor" />
            </svg>
            <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none">
              <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.7" />
              <path d="M12 11v5" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" />
              <circle cx="12" cy="7.9" r="1.05" fill="currentColor" />
            </svg>
          </span>
          <span class="toast-message">{{ item.message }}</span>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.toast-host {
  position: fixed;
  top: max(16px, env(safe-area-inset-top));
  right: 16px;
  z-index: 2000;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
  pointer-events: none;
  max-width: min(380px, calc(100vw - 32px));
}

.toast {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  padding: 10px 13px;
  border-radius: 10px;
  background: var(--bg-elev);
  border: 1px solid var(--border);
  box-shadow: var(--shadow-md);
  color: var(--text);
  font-size: 13px;
  line-height: 1.45;
  pointer-events: auto;
  cursor: pointer;
  max-width: 100%;
}

.toast-icon {
  flex: none;
  margin-top: 1px;
  color: var(--text-2);
}

.toast-success .toast-icon {
  color: var(--success);
}

.toast-error .toast-icon {
  color: var(--danger);
}

.toast-info .toast-icon {
  color: var(--primary);
}

.toast-message {
  word-break: break-word;
}

.toast-enter-active,
.toast-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

@media (max-width: 640px) {
  .toast-host {
    top: auto;
    bottom: calc(16px + env(safe-area-inset-bottom));
    left: 16px;
    right: 16px;
    align-items: stretch;
    max-width: none;
  }
}
</style>
