#include "cpp/apps/echo/srv.h"
#include "cpp/proc/proc.h"
#include "cpp/proxy/sigmap/sigmap.h"
#include "cpp/rpc/srv.h"
#include "cpp/serr/serr.h"
#include "cpp/sigmap/const.h"
#include "cpp/util/log/log.h"

#include <iostream>

int main(int argc, char *argv[]) {
  sigmaos::util::log::init_logger(sigmaos::apps::echo::ECHOSRV);
  log(sigmaos::apps::echo::ECHOSRV, "main");
  auto sp_clnt = std::make_shared<sigmaos::proxy::sigmap::Clnt>();

  // Create the echo server
  auto srv = std::make_shared<sigmaos::apps::echo::Srv>(sp_clnt);
  // Run the server
  srv->Run();
}
