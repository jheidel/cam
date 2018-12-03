import { PolymerElement } from '@polymer/polymer/polymer-element.js';
import '@polymer/paper-card/paper-card.js';
import { html } from '@polymer/polymer/lib/utils/html-tag.js';
/**
 * @customElement
 * @polymer
 */
class CamLive extends PolymerElement {
  static get template() {
    return html`
    <style>
      h2 {
        padding-left: 10px;
      }
      .card {
        padding: 10px;
        margin: 10px;
      }
      .fit {
              max-width: 100%;
              max-height: 100%;
      }
    </style>
    <h2>Live View</h2>
    <paper-card class="card">
       <img src="/mjpeg?name=default" class="fit">
    </paper-card>
`;
  }

  static get is() { return 'cam-live'; }
  static get properties() {
    return {
    };

  }
}
window.customElements.define(CamLive.is, CamLive);
