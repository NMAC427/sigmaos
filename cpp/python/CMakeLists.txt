message(STATUS "Building python clntlib library")

pybind11_add_module(_clntlib clntlib.cc)

target_link_libraries(_clntlib PRIVATE sigmap_proxy)