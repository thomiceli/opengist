import jdenticon from 'jdenticon/standalone';

// Render identicons for users without a real avatar. The server emits an empty
// <svg data-jdenticon-value="username"> and jdenticon fills it in on the client.
//
// update() reads the value attribute and (re)writes the SVG's children, so it is
// idempotent — safe to call again after each hx-boost swap without doubling up.
export function initJdenticon() {
  jdenticon.update('[data-jdenticon-value]');
}
