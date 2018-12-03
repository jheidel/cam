import { PolymerElement } from '@polymer/polymer/polymer-element.js';
import '@polymer/app-layout/app-drawer-layout/app-drawer-layout.js';
import '@polymer/app-layout/app-drawer/app-drawer.js';
import '@polymer/app-layout/app-header-layout/app-header-layout.js';
import '@polymer/app-layout/app-header/app-header.js';
import '@polymer/app-layout/app-scroll-effects/app-scroll-effects.js';
import '@polymer/app-layout/app-toolbar/app-toolbar.js';
import '@polymer/font-roboto/roboto.js';
import '@polymer/iron-ajax/iron-ajax.js';
import '@polymer/iron-icon/iron-icon.js';
import '@polymer/iron-icons/av-icons.js';
import 'iron-lazy-pages/iron-lazy-pages.js';
import '@polymer/paper-button/paper-button.js';
import '@polymer/paper-dialog/paper-dialog.js';
import '@polymer/paper-icon-button/paper-icon-button.js';
import '@polymer/paper-item/paper-icon-item.js';
import '@polymer/paper-listbox/paper-listbox.js';
import '@polymer/paper-toast/paper-toast.js';
import '../cam-live/cam-live.js';
import '../cam-events/cam-events.js';
import { html } from '@polymer/polymer/lib/utils/html-tag.js';
/**
 * @customPolymerElement
 * @polymer
 */
class WebApp extends PolymerElement {
  static get template() {
    return html`
    <style>
      :host {
        display: block;
      }
		body {
			margin: 0;
		  font-family: 'Roboto', 'Noto', sans-serif;
			background-color: #eee;
		}
		app-header {
			position: fixed;
			top: 0;
			left: 0;
			width: 100%;
			background-color: #4CAF50;
			color: #fff;
		}
    .nolink {
      text-decoration: none;
      color: inherit;
    }

    .headerpad {
            height:64px;
    }

    .title {
            display: flex;
            align-items: center;
    }
    .title > iron-icon {
            padding-right: 10px;
    }

    .vidwrap {
        /* TODO: better */
        width: calc(100vw - 400px);
        height: calc(100vh - 200px);

        overflow: none;
        display: flex;
        align-items: center;
        justify-content: center;
    }
      #video {
        object-fit: cover;
        max-width: 100%;
        max-height: 100%;
        display: inline-block;
      }

    </style>

<app-header-layout fullbleed="">
  <app-header slot="header" fixed="" condenses="" effects="waterfall">
    <app-toolbar>
      <paper-icon-button icon="menu" on-tap="toggleDrawer_"></paper-icon-button>
      <div class="title" main-title="">
              Security Camera Monitor
      </div>
    </app-toolbar>
  </app-header>
  <app-drawer-layout id="drawerlayout">
        <app-drawer id="drawer" slot="drawer" swipe-open="">
                <div class="headerpad"></div>
                <paper-listbox selected="{{route}}" attr-for-selected="data-route">
                        <paper-icon-item data-route="live">
                                <iron-icon icon="av:videocam" slot="item-icon"></iron-icon>
                                Live View
                        </paper-icon-item>
                        <paper-icon-item data-route="events">
                                <iron-icon icon="history" slot="item-icon"></iron-icon>
                                Event History
                        </paper-icon-item>
                </paper-listbox>
        </app-drawer>
        <div>
          <iron-lazy-pages selected="[[route]]" attr-for-selected="data-route">
                  <template is="dom-if" data-route="live">
                          <cam-live></cam-live>
                  </template>
                  <template is="dom-if" data-route="events">
                          <cam-events on-open-event="onOpenEvent_"></cam-events>
                  </template>
          </iron-lazy-pages>
        </div>
  </app-drawer-layout>
</app-header-layout>
<!-- TODO: move this to a separate element -->
<paper-dialog id="dialog" with-backdrop="">
        <div class="vidwrap">
            <video id="video" autoplay="" controls="" controlslist="nodownload">
                     HTML5 video tag unsupported.
            </video>
        </div>
        <div class="buttons">
          <paper-button on-tap="openDelete_">
                  <iron-icon icon="delete"></iron-icon>
                  Delete
          </paper-button>
          <paper-button on-tap="openFullscreen_">
                  <iron-icon icon="fullscreen"></iron-icon>
                  Fullscreen
          </paper-button>
          <a class="nolink" href="/video?id=[[event.ID]]&amp;download=true">
                  <paper-button>
                          <iron-icon icon="file-download"></iron-icon>
                          Download
                  </paper-button>
          </a>
          <paper-button dialog-dismiss="">
                  <iron-icon icon="close"></iron-icon>
                  Close
          </paper-button>
        </div>
</paper-dialog>
<iron-ajax id="deleteajax" url="/delete" method="POST" on-response="onDeleteResponse_"></iron-ajax>
<paper-dialog id="delete" modal="" always-on-top="" on-iron-overlay-closed="onDeleteClosed_">
        <div>
                <div>
                        About to permanently delete event [[event.ID]].
                </div>
                <div>
                        <b>Are you sure?</b> This action cannot be undone.
                </div>
        </div>
        <div class="buttons">
                <paper-button dialog-dismiss="" autofocus="">Cancel</paper-button>
                <paper-button dialog-confirm="">Delete</paper-button>
        </div>
</paper-dialog>
<paper-toast id="deletetoast">Successfully deleted event [[event.ID]].</paper-toast>
`;
  }

  static get is() { return 'web-app'; }
  static get properties() {
    return {
            route: {
                    type: String,
                    value: "events"
            },
            event: {
                    type: Object,
                    value: null,
            }
    };
  }

  onOpenEvent_(e) {
          this.event = e.detail.event;
          this.$.video.src = "/video?id=" + this.event.ID;
          this.$.dialog.open();
  }

  openDelete_(e) {
          this.$.delete.open();
  }

  onDeleteClosed_(e) {
          if (!e.detail.confirmed) {
                  return;
          }

          this.$.dialog.close();
          this.$.deleteajax.params = {
                  "id": this.event.ID,
          };
          this.$.deleteajax.generateRequest();
  }

  onDeleteResponse_(e) {
          this.$.deletetoast.show();
  }

  openFullscreen_(e) {
          this.$.video.webkitEnterFullScreen();
  }

  toggleDrawer_() {
          // TODO: doesn't work correctly on mobile.
          this.$.drawerlayout.forceNarrow = !this.$.drawerlayout.forceNarrow;
  }
}

window.customElements.define(WebApp.is, WebApp);
