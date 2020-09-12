import { PolymerElement } from '@polymer/polymer/polymer-element.js';
import '@polymer/iron-ajax/iron-ajax.js';
import '@polymer/iron-flex-layout/iron-flex-layout.js';
import '@polymer/iron-icon/iron-icon.js';
import '@polymer/iron-icons/iron-icons.js';
import '@polymer/iron-list/iron-list.js';
import '@polymer/paper-card/paper-card.js';
import '@polymer/paper-checkbox/paper-checkbox.js';
import '@polymer/paper-spinner/paper-spinner.js';
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
      .placeholder {
          padding: 10px;
          margin-left: 20px;
      }
      #empty {
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
      .options {
        padding-left: 15px;
        padding-bottom: 10px;
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

    <div class="options">
      <paper-checkbox checked="{{haveClassification_}}">Only Show Events with Detections</paper-checkbox>
    </div>

    <iron-ajax loading="{{loading_}}" id="ajax" url="/events" params="[[buildParams_(haveClassification_)]]" last-response="{{response}}" handle-as="json" auto=""></iron-ajax>

    <div hidden\$="[[!loading_]]" class="placeholder">
      <paper-spinner active></paper-spinner>
    </div>

    <div hidden\$="[[loading_]]">
      <div id="empty" hidden\$="[[!empty_(response.Items)]]" class="placeholder">
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
    </div>
`;
  }

  static get is() { return 'cam-events'; }
  static get properties() {
    return {
            response: {
                    type: Object,
                    value: null,
            },
            haveClassification_: {
                    type: Boolean,
                    value: true,
            },
            loading_: {
                    type: Boolean,
                    value: false,
            }
    };
  }

  buildParams_(haveClassification) {
    let params = {};
    if (haveClassification) {
      params["have_classification"] = "true";
    }
    return params;
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
