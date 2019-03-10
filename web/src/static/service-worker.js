'use strict';

// Modified from chrome web push demo: 
// https://github.com/GoogleChrome/samples/blob/gh-pages/push-messaging-and-notifications/service-worker.js

self.addEventListener('push', function(event) {
  console.log('(v6) Received a push message');
  console.log(event);

  const notification = event.data.json();
  if (!notification) {
    console.log("Failed to decode notification");
  }

  console.log("Notification object");
  console.log(notification);

  const cls = notification.Detection.Class;
  const tcls = cls.charAt(0).toUpperCase() + cls.slice(1);

  const pcnt = Math.round(notification.Detection.Confidence * 100) + "%";
  const ts = notification.TimeString;

  const title = `${tcls} detected!`;
  const body = `At ${ts} the security camera detected a ${cls} (confidence ${pcnt}).`

  event.waitUntil(
    self.registration.showNotification(title, {
      body: body,
      tag: notification.Identifier,
      image: '/thumb?id=' + notification.Identifier,
      icon: '/favicon.ico',
    })
  );
});

self.addEventListener('notificationclick', function(event) {
  console.log('On notification click: ', event.notification.tag);
  // Android doesn’t close the notification when you click on it
  // See: http://crbug.com/463146
  event.notification.close();

  // This looks to see if the current is already open and
  // focuses if it is
  event.waitUntil(clients.matchAll({
    type: 'window'
  }).then(function(clientList) {
    const targetUrl = '/#live'
    for (var i = 0; i < clientList.length; i++) {
      var client = clientList[i];
      if (client.url === targetUrl && 'focus' in client) {
        return client.focus();
      }
    }
    if (clients.openWindow) {
      return clients.openWindow(targetUrl);
    }
  }));
});
