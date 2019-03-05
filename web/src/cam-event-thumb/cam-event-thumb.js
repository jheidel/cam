import { PolymerElement } from '@polymer/polymer/polymer-element.js';
import '@polymer/iron-icons/iron-icons.js';
import '@polymer/iron-icons/maps-icons.js';
import '@polymer/iron-icons/social-icons.js';
import '@polymer/paper-card/paper-card.js';
import { html } from '@polymer/polymer/lib/utils/html-tag.js';
import moment from 'moment/src/moment.js';
/**
 * @customElement
 * @polymer
 */
class CamEventThumb extends PolymerElement {
  static get template() {
    return html`
    <style>
       .thumb-container {
          position: relative;
       }
      .thumbbox {
        background-color: black;
        background-size: cover;
        display: flex;
        align-items: center;
        justify-content: center;
        color: white;
        font-weight: bold;
      }
      .thumbsize {
        width: 320px;
        height: 180px;
      }
      .tlink {
        text-decoration: none;
        color: inherit;
      }
      .belowthumb {
              display: flex;
              justify-content: space-between;
              flex-direction: row;
              align-items: center;
              padding-left: 5px;
              padding-right: 5px;
      }
      .time {
              font-size: large;
      }
      .small {
              font-size: x-small;
      }
      .duration {
              color: #888;
              text-align: right;
      }
      .detection {
              display: flex;
              background-color: #fef;
              position: absolute;
              bottom: 0;
              right: 0;
              padding: 2px;
      }
    </style>
    <paper-card>
    <div on-tap="eventClicked_" class="thumb-container">
      <a class="tlink" href="javascript:void(0);">
        <div id="tbox" class="thumbbox thumbsize" on-mouseover="hoverVideo_" on-mouseout="hideVideo_">
          <video id="vthumb" class="thumbsize" loop="" preload="none" autoplay="" hidden="true">
                  HTML5 video tag unsupported.
          </video>
          <template is="dom-if" if="[[showEmpty_(event)]]">
            <div>Preview Unavailable</div>
          </template>
        </div>
      </a>

       <dom-if if="[[event.Detection]]">
                <template>
                        <div class="detection">
                                <iron-icon icon="[[computeIcon_(event.Detection.Class)]]"></iron-icon>
                                <span>[[computePercent_(event.Detection.Confidence)]]</span>
                        </div>
                </template>
        </dom-if>
      </div>

      <div class="belowthumb">
              <div>
                      <div class="date">
                        [[formatAsDate_(event.Timestamp)]]
                      </div>
                      <div class="time">
                        [[formatAsTime_(event.Timestamp)]]
                      </div>
              </div>
              <div class="duration" hidden\$="[[!event.HaveVideo]]">
                      <div class="small">
                              Duration
                      </div>
                      <div>
                              [[formatDuration_(event.DurationSec)]]
                      </div>
              </div>
      </div>
    </paper-card>
`;
  }

  static get is() { return 'cam-event-thumb'; }
  static get properties() {
    return {
            event: {
                    type: Object,
                    value: null,
                    observer: 'eventChanged_',
            }
    };
  }

  formatDuration_(sec) {
          const m = Math.trunc(sec / 60);
          const s = sec - m * 60;
          return m + ":" + (s < 10 ? "0" : "") + s;
  }

  formatAsDate_(tsec) {
          return moment.unix(tsec).format("dddd, MMM D, YYYY");
  }
  formatAsTime_(tsec) {
          return moment.unix(tsec).format("h:mm A");
  }

  eventClicked_() {
          if (this.event.HaveVideo) {
                  this.dispatchEvent(new CustomEvent('open-event', {detail: {event: this.event}, bubbles: true, composed: true}));
          }
  }

  computeIcon_(c) {
          if (c === "person") {
                  return "social:person";
          }
          if (c === "vehicle") {
                  return "maps:directions-car";
          }
          if (c === "animal") {
                  return "pets";
          }
          return "help";
  }

  computePercent_(p) {
        return Math.round(p * 100) + "%";
  }

  eventChanged_(newValue, oldValue) {
          if (!!newValue && newValue.HaveThumb) {
                  this.$.tbox.style.backgroundImage = "url(/thumb?id=" + newValue.ID + ")";
          } else {
                  this.$.tbox.style.backgroundImage = "";
                        }
  }

  hoverVideo_() {
          if (!this.showVThumb_(this.event)) {
                            return;
                        }
          this.$.vthumb.src = "/vthumb?id=" + this.event.ID;
                        this.$.vthumb.hidden = false;
  }

  hideVideo_() {
          if (!this.showVThumb_(this.event)) {
                            return;
                        }
                        this.$.vthumb.hidden = true;
          this.$.vthumb.src = "";
  }

  showVThumb_(e) {
          if (!e) {
                  return false;
          }   
          return e.HaveVThumb;
  }

  showEmpty_(e) {
          if (!e) {
                  return false;
          }   
          return !e.HaveThumb;
  }
}

window.customElements.define(CamEventThumb.is, CamEventThumb);
