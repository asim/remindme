# Remind Me

Location based API used as the basis of a reminder app.

```
        POST /contacts -- add contact to a users contact list
        request: {id: user_id, contacts: [ contact1, contact2, ... ]}

        POST /ping -- update user location
        request: {id: user_id, location: {lat: lat, lon: lon, alt: altitude}}

        POST /near -- get nearby contacts
        request: {id: user_id, location: {lat: lat, lon: lon, alt: altitude}}
        response: [ contact1, contact2, ... ]
```
