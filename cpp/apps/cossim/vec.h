#pragma once

#include "cpp/apps/cache/clnt.h"
#include "apps/cossim/proto/cossim.pb.h"
#include "cpp/io/conn/conn.h"
#include "cpp/io/conn/tcp/tcp.h"
#include "cpp/io/demux/srv.h"
#include "cpp/io/net/srv.h"
#include "cpp/io/transport/transport.h"
#include "cpp/proxy/sigmap/sigmap.h"
#include "cpp/rpc/srv.h"
#include "cpp/serr/serr.h"
#include "cpp/sigmap/const.h"
#include "sigmap/sigmap.pb.h"
#include "cpp/util/log/log.h"
#include "cpp/util/perf/perf.h"

#include <cmath>
#include <expected>
#include <filesystem>
#include <format>
#include <limits>
#include <memory>
#include <vector>

namespace sigmaos {
namespace apps::cossim {

class Vector {
 public:
  Vector(::Vector *proto_vec, int dim)
      : _proto_vec(proto_vec),
        _underlying_buf(nullptr),
        _vals(nullptr),
        _dim(dim) {}
  Vector(std::shared_ptr<std::string> underlying_buf, char *vals, int dim)
      : _proto_vec(nullptr),
        _underlying_buf(underlying_buf),
        _vals((double *)vals),
        _dim(dim) {}
  ~Vector() {}

  double Get(int idx) const;
  double CosineSimilarity(std::shared_ptr<Vector> other) const;

 private:
  ::Vector *_proto_vec;
  std::shared_ptr<std::string> _underlying_buf;
  double *_vals;
  int _dim;
};

};  // namespace apps::cossim
};  // namespace sigmaos
