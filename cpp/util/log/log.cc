#include <util/log/log.h>

#include <iostream>
#include <unordered_set>

namespace sigmaos {
namespace util::log {

Logger::Logger(const std::string& selector) : _selector(selector) {
  std::string sigmadebug(sigmaos::util::common::get_env("SIGMADEBUG"));
  std::string pid(sigmaos::util::common::get_env("SIGMADEBUGPID"));

  _pid = pid.empty() ? "0" : pid;

  if (selector == ALWAYS || selector == FATAL) {
    _enabled = true;
  } else {
    _enabled = sigmaos::util::common::ContainsLabel(sigmadebug, selector);
  }
}

bool _log::_l_always = init_logger(ALWAYS);
bool _log::_l_fatal = init_logger(FATAL);
bool _log::_l_test = init_logger(TEST);
bool _log::_l_spawn_lat = init_logger(SPAWN_LAT);
bool _log::_l_proxy_rpc_lat = init_logger(PROXY_RPC_LAT);

_log::_log() {}
_log::~_log() {}

std::mutex _mutex;
bool init_logger(std::string selector) {
  std::lock_guard<std::mutex> guard(_mutex);

  // If this logger hasn't already been initialized, create a new one and
  // register it.
  auto& registry = LoggerRegistry::instance();
  auto existing_logger = registry.get_logger(selector);
  if (!existing_logger) {
    auto logger = std::make_shared<Logger>(selector);
    registry.register_logger(selector, logger);
    return true;
  }
  return false;
}

};  // namespace util::log
};  // namespace sigmaos
