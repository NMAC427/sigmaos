#include <util/log/log.h>

#include <iostream>

namespace sigmaos {
namespace util::log {

bool _log::_l_always = init_logger(ALWAYS);
bool _log::_l_fatal = init_logger(FATAL);
bool _log::_l_test = init_logger(TEST);
bool _log::_l_spawn_lat = init_logger(SPAWN_LAT);

bool init_logger(std::string selector) {
  static std::mutex _mu;
  std::lock_guard<std::mutex> guard(_mu);
  auto log = spdlog::get(selector);
  // If this logger hasn't already been initialized, create a new one and
  // register it.
  if (!log) {
    auto sdbg_sink =
        std::make_shared<sigmaos::util::log::sigmadebug_sink>(selector);
    log = std::make_shared<spdlog::logger>(selector, sdbg_sink);
    spdlog::register_logger(log);
    return true;
  }
  return false;
}

};  // namespace util::log
};  // namespace sigmaos
