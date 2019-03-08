import { PolymerElement } from '@polymer/polymer/polymer-element.js';
import '@polymer/iron-ajax/iron-ajax.js';
import '@polymer/paper-toggle-button/paper-toggle-button.js';
import { html } from '@polymer/polymer/lib/utils/html-tag.js';
/**
 * @customPolymerElement
 * @polymer
 */
class CamNotifications extends PolymerElement {
  static get template() {
    return html`
    <style>
      paper-toggle-button {
        padding-top: 10px;
      }
      .help {
        font-size: 10pt;
      }
      [hidden] {
              display: none;
      }
    </style>
    <iron-ajax
      auto
      url="/push_get_pubkey"
      handle-as="text"
      last-response="{{pubkey}}"
    ></iron-ajax>
    <iron-ajax
      id="ajaxsub"
      method="post"
      handle-as="json"
      content-type="application/json"
      on-error="subscribeError_"
    ></iron-ajax>
    <div hidden$="[[!notificationsSupported_()]]">
        <div class="help">
                Enable push notifications to get updates on
                motion events. You will need to subscribe on
                each browser where you want to be notified.
        </div>
        <paper-toggle-button id="toggle" disabled>Push Notifications</paper-toggle-button>
    </div>
`;
  }

  static get is() { return 'cam-notifications'; }
  static get properties() {
    return {
            pubkey: {
                    type: String,
            },
    };
  }

        notificationsSupported_() {
                if (!('serviceWorker' in navigator)) {
                        return false;
                }
                if (!('PushManager' in window)) {
                        return false;
                }
                return true;
        }

        askPermission_() {
                return new Promise(function(resolve, reject) {
                        const permissionResult = Notification.requestPermission(function(result) {
                                resolve(result);
                        });
                        if (permissionResult) {
                                permissionResult.then(resolve, reject);
                        }
                })
                .then(function(permissionResult) {
                        if (permissionResult !== 'granted') {
                        throw new Error('We weren\'t granted permission.');
                        }
                });
        }

        urlB64ToUint8Array_(base64String) {
                const padding = '='.repeat((4 - base64String.length % 4) % 4);
                const base64 = (base64String + padding)
                  .replace(/\-/g, '+')
                  .replace(/_/g, '/');
              
                const rawData = window.atob(base64);
                const outputArray = new Uint8Array(rawData.length);
              
                for (let i = 0; i < rawData.length; ++i) {
                  outputArray[i] = rawData.charCodeAt(i);
                }
                return outputArray;
        }

        subscribeUserToPush_() {
                const pubkey = this.pubkey;
                console.log("PUBKEY " + pubkey);
                return navigator.serviceWorker.register('service-worker.js')
                .then((registration)  => {
                  console.log("Generating options");
                  const subscribeOptions = {
                    userVisibleOnly: true,
                    applicationServerKey: this.urlB64ToUint8Array_(pubkey)
                  };

                  console.log(subscribeOptions);
              
                  return registration.pushManager.subscribe(subscribeOptions);
                })
                .then((pushSubscription) => {
                  console.log('Received PushSubscription: ', JSON.stringify(pushSubscription));

                  this.$.ajaxsub.url = "/push_subscribe";
                  this.$.ajaxsub.body = JSON.stringify(pushSubscription);
                  this.$.ajaxsub.generateRequest();
                  return pushSubscription;
                });
        }

        subscribeError_(e) {
                // Error with POST, could be a subscribe or unsubscribe.
                console.log("POST subscribe error");
                console.log(e);
        }

        subscribe_() {
                if (!this.pubkey) {
                        console.log("Missing pubkey!");
                        return;
                }

                this.askPermission_().then((result) => {
                        console.log("Subscribing to push...");
                        this.subscribeUserToPush_().then((subscribe) => {
                                console.log("GOT SUBSCRIPTION!");
                                console.log(subscribe);

                                // TODO: post this along to the server!
                        }, (err) => {
                                console.log("Failed to register push");
                                console.log(err.message);
                        });
                }, (err) => {
                        console.log("Permission rejected: " + err);
                });
        }

        unsubscribe_() {
                navigator.serviceWorker.ready
                .then(reg => reg.pushManager.getSubscription())
                .then(sub => {
                        console.log("ACTIVE PUSH IS");
                        console.log(sub);

                  this.$.ajaxsub.url = "/push_unsubscribe";
                  this.$.ajaxsub.body = JSON.stringify(sub);
                  this.$.ajaxsub.generateRequest();

                        return sub.unsubscribe();
                })
                .then(success => {
                        console.log("Unsubscription successful.");
                }, (err) => {
                        // TODO better error propagation.
                        console.log("Failed to unsubscribe.");
                        console.log(err);
                        console.log(err.message);
                });
        }

        onNotificationToggle_(e) {
                const enabled = e.target.checked;
                if (enabled) {
                        this.subscribe_();
                } else {
                        this.unsubscribe_();
                }
        }

        async hasSubscription_() {
                try {
                     const haveSub = navigator.serviceWorker.register('service-worker.js')
                        .then(reg => reg.pushManager.getSubscription())
                        .then(sub => !!sub);
                        return await haveSub;
                } catch(e) {
                        console.log("Failed to look up subscription");
                        console.log(e);
                        return false;
                }
        }

        ready() {
                super.ready();
                if (!this.notificationsSupported_()) {
                        return;
                }
                this.hasSubscription_().then((subscribed) => {
                        this.$.toggle.checked = subscribed;
                        this.$.toggle.disabled = false;
                        this.$.toggle.addEventListener('change', e => this.onNotificationToggle_(e));
                });
        }
}

window.customElements.define(CamNotifications.is, CamNotifications);
