import { PolymerElement } from '@polymer/polymer/polymer-element.js';
import '@polymer/iron-ajax/iron-ajax.js';
import '@polymer/iron-flex-layout/iron-flex-layout.js';
import '@polymer/iron-icon/iron-icon.js';
import '@polymer/iron-icons/iron-icons.js';
import '@polymer/iron-list/iron-list.js';
import '@polymer/paper-card/paper-card.js';
import '../cam-event-thumb/cam-event-thumb.js';
import { html } from '@polymer/polymer/lib/utils/html-tag.js';
import moment from 'moment/src/moment.js';

/**
 * @customElement
 * @polymer
 */
class CamEvents extends PolymerElement {
  static get template() {
    return html`
    <style>
      .item {
        padding: 10px;
      }
      #empty {
              padding: 10px;
              margin-left: 20px;
              color: #666;
              background-color: #ddd;
              display: inline-flex;
              align-items: center;
              justify-content: center;
      }
      #empty > iron-icon {
              padding-right: 5px;
      }
      /* Allow hidden behavior to work with flexbox */
      #empty[hidden] {
              display: none;
      }
      .header {
        padding-left: 10px;
        padding-right: 10px;
        padding-top: 10px;

        display: flex;
        justify-content: space-between;
      }
      .header > div {
        font-size: small;
      }
      th {
              text-align: left;
      }
    </style>
    <div class="header">
            <h2>Event History</h2>
            <div hidden\$="[[empty_(response.Items)]]">
                    <table>
                            <tbody><tr>
                                    <th>Recorded</th>
                                    <td>[[formatCount_(response.ItemsCount, 'event', 'events')]]</td>
                            </tr>
                            <tr>
                                    <th>Total Size</th>
                                    <td>[[formatBytes_(response.ItemsTotalSize)]]</td>
                            </tr>
                            <tr>
                                    <th>Oldest</th>
                                    <td>[[formatTimestamp_(response.OldestTimestamp)]]</td>
                            </tr>
                    </tbody></table>
            </div>
    </div>


    <iron-ajax id="ajax" url="/events" last-response="{{response}}" handle-as="json" auto=""></iron-ajax>
    <div id="empty" hidden\$="[[!empty_(response.Items)]]">
          <iron-icon icon="info"></iron-icon>
          No events recorded.
    </div>
    <iron-list items="[[response.Items]]" as="item" grid="" scroll-target="document">
      <template>
        <div class="item">
          <cam-event-thumb event="[[item]]"></cam-event-thumb>
        </div>
      </template>
    </iron-list>
`;
  }

  static get is() { return 'cam-events'; }
  static get properties() {
    return {
            response: {
                    type: Object,
                    value: null,
            }
    };
  }

  ready() {
    super.ready();

    // Automatically refresh page when new events are triggered.
    this.ws = new WebSocket("wss://" + window.location.host + "/eventsws");
    const ajax = this.$.ajax;
    this.ws.onmessage = function(e) {
          ajax.generateRequest();
    };
  }

  empty_(l) {
    if(!Array.isArray(l)) {
            return true;
    }
    return l.length === 0;
  }

  formatTimestamp_(tsec) {
          return moment.unix(tsec).format("M/D/YYYY h:mm A");
  }

  formatBytes_(bytes) {
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    if (bytes == 0) {
      return '0 B';
    }
    const i = parseInt(Math.floor(Math.log(bytes) / Math.log(1024)));
    return Math.round(100 * bytes / Math.pow(1024, i)) / 100 + ' ' + sizes[i];
  }

  formatCount_(n, s, p) {
          if (!n) {
                  n = 0;
          }
          const lbl = n == 1 ? s : p;
          return n.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",") + " " + lbl;
  }
}

window.customElements.define(CamEvents.is, CamEvents);
