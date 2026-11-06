#pragma once
// Text helpers for d200.yaml's keys (esphome: includes)
#include <algorithm>
#include <cmath>
#include <string>
#include "esphome/components/json/json_util.h"
#include "esphome/core/helpers.h"
#include "esphome/core/time.h"

// "8,4 mm": one decimal with the Polish decimal comma, then unit; "--" without a value
inline std::string wx_dec(float v, const char *unit) {
  std::string s = std::isnan(v) ? std::string("--") : esphome::str_sprintf("%.1f", v);
  auto dot = s.find('.');
  if (dot != std::string::npos)
    s[dot] = ',';
  return s + unit;
}

// "3,7°C"
inline std::string wx_deg(float t) { return wx_dec(t, "°C"); }

// "13:05", the local time of a unix timestamp; "--:--" for none
inline std::string wx_hhmm(int ts) {
  if (ts <= 0)
    return "--:--";
  return esphome::ESPTime::from_epoch_local(ts).strftime("%H:%M");
}

// A time span: "45 min", "2 h 05", "30 h", "4 dni"
inline std::string wx_span(int secs) {
  int m = (std::max(secs, 0) + 59) / 60;
  if (m < 60)
    return esphome::str_sprintf("%d min", m);
  if (m < 10 * 60)
    return esphome::str_sprintf("%d h %02d", m / 60, m % 60);
  if (m < 48 * 60)
    return esphome::str_sprintf("%d h", (m + 30) / 60);
  return esphome::str_sprintf("%d dni", (m + 12 * 60) / (24 * 60));
}

// What HA rendered from a response_template: it arrives under "response", as an object or as
// a JSON string (parsed into doc). Unbound when it is neither
inline ArduinoJson::JsonObjectConst wx_unwrap(ArduinoJson::JsonObjectConst response, ArduinoJson::JsonDocument &doc) {
  auto r = response["response"];
  if (r.is<const char *>()) {
    doc = esphome::json::parse_json(std::string(r.as<const char *>()));
    return doc.as<ArduinoJson::JsonObjectConst>();
  }
  return r.as<ArduinoJson::JsonObjectConst>();
}

// Whether the next sun event after now is the sunset (else the sunrise)
inline bool wx_sunset_next(int now, int sunset, int sunrise) {
  bool s = sunset > now, r = sunrise > now;
  return s && (!r || sunset < sunrise);
}
