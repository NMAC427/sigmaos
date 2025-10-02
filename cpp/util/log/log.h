#pragma once

#include <util/common/util.h>

#include <chrono>
#include <fmt/format.h>
#include <iostream>
#include <iomanip>
#include <memory>
#include <mutex>
#include <string>
#include <unordered_map>

// Some common debug selectors
const std::string TEST = "TEST";
const std::string ALWAYS = "ALWAYS";
const std::string FATAL = "FATAL";
const std::string SPAWN_LAT = "SPAWN_LAT";
const std::string PROXY_RPC_LAT = "PROXY_RPC_LAT";

namespace sigmaos {
namespace util::log {

// Initialize a logger with a debug selector
bool init_logger(std::string selector);

const std::string ERR = "_ERR";

class Logger {
 public:
  Logger(const std::string& selector);

  template <typename... Args>
  void info(fmt::format_string<Args...> fmt, Args &&...args) {
    if (_enabled) {
      log_message(fmt, std::forward<Args>(args)...);
    }
  }

  void flush() {
    std::cout.flush();
  }

 private:
  template <typename... Args>
  void log_message(fmt::format_string<Args...> fmt, Args &&...args) {
    std::lock_guard<std::mutex> lock(_mutex);

    // Get current time
    auto now = std::chrono::system_clock::now();
    auto time_t = std::chrono::system_clock::to_time_t(now);
    auto tm = *std::localtime(&time_t);

    // Get microseconds
    auto duration = now.time_since_epoch();
    auto micros = std::chrono::duration_cast<std::chrono::microseconds>(duration) % 1000000;

    // Format timestamp and message
    std::string formatted_msg = fmt::format(fmt, std::forward<Args>(args)...);

    std::cout << std::setfill('0')
              << std::setw(2) << tm.tm_hour << ":"
              << std::setw(2) << tm.tm_min << ":"
              << std::setw(2) << tm.tm_sec << "."
              << std::setw(6) << micros.count() << " "
              << _pid << " " << _selector << " "
              << formatted_msg << std::endl;
  }

  bool _enabled;
  std::string _selector;
  std::string _pid;
  std::mutex _mutex;
};

// Logger registry
class LoggerRegistry {
 public:
  static LoggerRegistry& instance() {
    static LoggerRegistry registry;
    return registry;
  }

  std::shared_ptr<Logger> get_logger(const std::string& selector) {
    std::lock_guard<std::mutex> lock(_mutex);
    auto it = _loggers.find(selector);
    if (it != _loggers.end()) {
      return it->second;
    }
    return nullptr;
  }

  void register_logger(const std::string& selector, std::shared_ptr<Logger> logger) {
    std::lock_guard<std::mutex> lock(_mutex);
    _loggers[selector] = logger;
  }

 private:
  std::mutex _mutex;
  std::unordered_map<std::string, std::shared_ptr<Logger>> _loggers;
};

// Used to initialize some common debug selectors
class _log {
 public:
  _log();
  ~_log();

 private:
  static bool _l_always;
  static bool _l_fatal;
  static bool _l_test;
  static bool _l_spawn_lat;
  static bool _l_proxy_rpc_lat;
};

};  // namespace util::log
};  // namespace sigmaos

// Write a log line given a selector
template <typename... Args>
void log(std::string selector, fmt::format_string<Args...> fmt, Args &&...args) {
  auto& registry = sigmaos::util::log::LoggerRegistry::instance();
  auto logger = registry.get_logger(selector);
  if (logger == nullptr) {
    sigmaos::util::log::init_logger(selector);
    logger = registry.get_logger(selector);
  }
  logger->info(fmt, std::forward<Args>(args)...);
}

// Write a log line given a selector
template <typename... Args>
[[noreturn]] void fatal(fmt::format_string<Args...> fmt, Args &&...args) {
  auto& registry = sigmaos::util::log::LoggerRegistry::instance();
  auto logger = registry.get_logger(FATAL);
  if (logger == nullptr) {
    sigmaos::util::log::init_logger(FATAL);
    logger = registry.get_logger(FATAL);
  }
  logger->info(fmt, std::forward<Args>(args)...);
  throw std::runtime_error("FATAL CPP");
}