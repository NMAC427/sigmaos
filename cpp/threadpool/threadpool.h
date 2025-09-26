#pragma once

#include "cpp/proxy/sigmap/sigmap.h"
#include "cpp/serr/serr.h"
#include "cpp/sigmap/const.h"
#include "sigmap/sigmap.pb.h"
#include "cpp/util/log/log.h"

#include <condition_variable>
#include <expected>
#include <format>
#include <memory>
#include <mutex>
#include <queue>
#include <thread>
#include <vector>

namespace sigmaos {
namespace threadpool {

const std::string THREADPOOL = "THREADPOOL";
const std::string THREADPOOL_ERR = THREADPOOL + sigmaos::util::log::ERR;

class Threadpool {
 public:
  Threadpool(std::string name) : Threadpool(name, 0) {}
  Threadpool(std::string name, int n_initial_threads)
      : _mu(), _cond(), _name(name), _n_idle(0), _threads(), _work_q() {
    for (int i = 0; i < n_initial_threads; i++) {
      add_thread();
    }
  }
  ~Threadpool() {}

  // Run a function in the threadpool
  void Run(std::function<void(void)> f);

 private:
  std::mutex _mu;
  std::condition_variable _cond;
  std::string _name;
  int _n_idle;
  std::vector<std::thread> _threads;
  std::queue<std::function<void(void)>> _work_q;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  // Start a new thread
  void add_thread();
  // Thread main loop
  void work();
};

};  // namespace threadpool
};  // namespace sigmaos
